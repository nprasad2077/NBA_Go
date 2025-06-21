// File: services/player_shot_chart_scrape_service.go
package services

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"github.com/nprasad2077/NBA_Go/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FetchAndStoreShotChartScrapedForPlayer scrapes the shot chart pages on
// Basketball-Reference for one player and batch upserts every shot.
func FetchAndStoreShotChartScrapedForPlayer(
	db *gorm.DB,
	playerID string,
	startSeason, endSeason int,
) error {
	// Loop newest → oldest season
	for season := startSeason; season >= endSeason; season-- {
		url := fmt.Sprintf(
			"https://www.basketball-reference.com/players/%s/%s/shooting/%d",
			playerID[:1], playerID, season,
		)

		// 1) HTTP GET the page content
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("request creation error for season %d: %w", season, err)
		}
		req.Header.Set("User-Agent",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
		)
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return fmt.Errorf("HTTP error for season %d: %w", season, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			// This is not an error, just means the player might not have data for that season.
			log.Printf("⚠️  Skipping season %d for player %s (Status: %s)", season, playerID, resp.Status)
			continue
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read error for season %d: %w", season, err)
		}

		// 2) Parse the player name (nice to have)
		fullDoc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("name-parse error for season %d: %w", season, err)
		}
		playerName := fullDoc.Find("#meta span[itemprop='name']").First().Text()
		if playerName == "" {
			playerName = playerID
		}

		// 3) Extract the shot chart HTML, which is hidden inside a comment
		shotHTML := extractCommentedShotChart(bodyBytes)
		if shotHTML == "" {
			log.Printf("⚠️  No shot-chart comment found for player %s in season %d", playerID, season)
			continue
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(shotHTML))
		if err != nil {
			return fmt.Errorf("snippet-parse error for season %d: %w", season, err)
		}
		wrapper := doc.Find("div#div_shot-chart div#shot-wrapper")
		if wrapper.Length() == 0 {
			log.Printf("⚠️  No shot-wrapper found for player %s in season %d", playerID, season)
			continue
		}

		// --- BATCHING LOGIC START ---
		// Create a slice to hold all the shot data for the current season.
		var shotsToUpsert []models.PlayerShotChart
		// >>>>>>>>>> FIX START: Add a map to track unique shots to prevent duplicates.
		uniqueShots := make(map[string]struct{})
		// <<<<<<<<<< FIX END

		// 4) Scrape every tooltip and collect the data into the slice.
		wrapper.Find("div.tooltip.make, div.tooltip.miss").Each(func(_ int, s *goquery.Selection) {
			// Position on the court
			style, _ := s.Attr("style")
			parts := strings.Split(style, ";")
			top := parsePx(parts[0])
			left := parsePx(parts[1])

			// Tooltip text
			tip, _ := s.Attr("tip")
			tipParts := strings.Split(tip, "<br>")

			// Date, team, opponent
			header := tipParts[0]
			dateSegs := strings.SplitN(header, ", ", 3)
			date := dateSegs[0] + "," + dateSegs[1]

			var team, opponent string
			if len(dateSegs) == 3 {
				game := dateSegs[2]
				if p := strings.SplitN(game, " at ", 2); len(p) == 2 {
					team, opponent = p[0], p[1]
				} else if p := strings.SplitN(game, " vs ", 2); len(p) == 2 {
					team, opponent = p[0], p[1]
				}
			}

			// Quarter & time remaining
			qt := strings.SplitN(tipParts[1], ",", 2)
			quarter := qt[0]
			timeRem := strings.Fields(qt[1])[0]

			// Result, shot type & distance
			rt := strings.Fields(tipParts[2])
			made := rt[0] == "Made"
			shotType := rt[1]
			distance := mustAtoi(rt[len(rt)-2])

			// Score & lead flag
			last := strings.Fields(tipParts[3])
			sc := strings.Split(last[len(last)-1], "-")
			teamScore, oppScore := mustAtoi(sc[0]), mustAtoi(sc[1])
			lead := teamScore > oppScore

			// >>>>>>>>>> FIX START: Create a unique key based on the conflict columns.
			uniqueKey := fmt.Sprintf("%s|%d|%s|%s|%s|%d|%d",
				playerID, season, date, quarter, timeRem, top, left)
			
			// If we have already seen this key, skip this iteration.
			if _, exists := uniqueShots[uniqueKey]; exists {
				return
			}
			uniqueShots[uniqueKey] = struct{}{}
			// <<<<<<<<<< FIX END


			shot := models.PlayerShotChart{
				PlayerID:          playerID,
				PlayerName:        playerName,
				Top:               top,
				Left:              left,
				Date:              date,
				Quarter:           quarter,
				TimeRemaining:     timeRem,
				Result:            made,
				ShotType:          shotType,
				DistanceFt:        distance,
				Lead:              lead,
				TeamScore:         teamScore,
				OpponentTeamScore: oppScore,
				Opponent:          opponent,
				Team:              team,
				Season:            season,
			}
			// Add the parsed shot object to our slice.
			shotsToUpsert = append(shotsToUpsert, shot)
		})

		// 5) Perform the batch upsert operation for the current season.
		if len(shotsToUpsert) > 0 {
			log.Printf("Attempting to batch upsert %d shots for player %s in season %d...", len(shotsToUpsert), playerID, season)

			// NOTE: I am assuming the column name for the "Quarter" field is "qtr".
			// If not, you must update the clause.OnConflict below.
			if err := db.Clauses(clause.OnConflict{
				Columns: []clause.Column{ // MUST match the unique index order in the model
					{Name: "player_id"}, {Name: "season"}, {Name: "date"},
					{Name: "qtr"}, {Name: "time_remaining"}, {Name: "top"}, {Name: "left"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"player_name", "result", "shot_type", "distance_ft",
					"lead", "team_score", "opponent_team_score",
					"opponent", "team",
				}),
			}).Create(&shotsToUpsert).Error; err != nil {
				// If the batch operation fails, log the error and return it.
				return fmt.Errorf("DB upsert error for player %s in season %d: %w", playerID, season, err)
			}

			log.Printf("✅ Successfully batch upserted %d shots for player %s in season %d.", len(shotsToUpsert), playerID, season)
		} else {
			log.Printf("No shots found to import for player %s in season %d.", playerID, season)
		}
		// --- BATCHING LOGIC END ---
	}
	return nil
}

// extractCommentedShotChart returns the inner HTML of the comment block that
// contains <div id="div_shot-chart" …>. BR hides the SVG there for ad reasons.
func extractCommentedShotChart(htmlBytes []byte) string {
	// ... (this function remains unchanged)
	root, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return ""
	}
	var found string
	var walker func(*html.Node)
	walker = func(n *html.Node) {
		if n.Type == html.CommentNode && strings.Contains(n.Data, `id="div_shot-chart"`) {
			found = n.Data
			return
		}
		for c := n.FirstChild; c != nil && found == ""; c = c.NextSibling {
			walker(c)
		}
	}
	walker(root)
	return found
}

// parsePx turns "left:244px" or "top:18px" into int(244 / 18).
func parsePx(s string) int {
	// ... (this function remains unchanged)
	parts := strings.Split(s, ":")
	return mustAtoi(strings.TrimSuffix(parts[1], "px"))
}