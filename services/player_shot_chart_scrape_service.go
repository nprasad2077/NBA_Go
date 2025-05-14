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

// FetchAndStoreShotChartScrapedForPlayer scrapes the shot chart pages on
// Basketball‑Reference for one player and upserts every shot into SQLite.
// A *composite* unique key keeps duplicates out:
//
//   (player_id, season, date, qtr, time_remaining, top, left)
//
// The model therefore needs a matching unique index (see models package).
func FetchAndStoreShotChartScrapedForPlayer(
	db *gorm.DB,
	playerID string,
	startSeason, endSeason int,
) error {
	// loop newest → oldest
	for season := startSeason; season >= endSeason; season-- {
		url := fmt.Sprintf(
			"https://www.basketball-reference.com/players/%s/%s/shooting/%d",
			playerID[:1], playerID, season,
		)

		// ─────────────────────── 1) HTTP GET  ────────────────────────
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("request creation error for %d: %w", season, err)
		}
		req.Header.Set("User-Agent",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
		)
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return fmt.Errorf("HTTP error for %d: %w", season, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("unexpected status for %d: %s", season, resp.Status)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read error for %d: %w", season, err)
		}

		// ─────────────────── 2) player name (nice to have) ───────────
		fullDoc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("name‑parse error for %d: %w", season, err)
		}
		playerName := fullDoc.Find("#meta span[itemprop='name']").First().Text()
		if playerName == "" {
			playerName = playerID
		}

		// ───────────────── 3) commented‑out shot chart HTML ──────────
		shotHTML := extractCommentedShotChart(bodyBytes)
		if shotHTML == "" {
			log.Printf("⚠️  no shot‑chart comment found for %d", season)
			continue
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(shotHTML))
		if err != nil {
			return fmt.Errorf("snippet‑parse error for %d: %w", season, err)
		}
		wrapper := doc.Find("div#div_shot-chart div#shot-wrapper")
		if wrapper.Length() == 0 {
			log.Printf("⚠️  no shot‑wrapper for %d", season)
			continue
		}

		// ───────────────────── 4) scrape every tooltip ───────────────
		var firstErr error // surface the first DB failure after the loop
		wrapper.Find("div.tooltip.make, div.tooltip.miss").Each(func(_ int, s *goquery.Selection) {
			// position on the court
			style, _ := s.Attr("style")
			parts := strings.Split(style, ";")
			top := parsePx(parts[0])
			left := parsePx(parts[1])

			// tooltip text
			tip, _ := s.Attr("tip")
			tipParts := strings.Split(tip, "<br>")

			// date, team, opponent
			header := tipParts[0]                       // e.g. "Oct 20, 2021, CHI at DET"
			dateSegs := strings.SplitN(header, ", ", 3) // {"Oct 20", "2021", "CHI at DET"}
			date := dateSegs[0] + "," + dateSegs[1]     // "Oct 20,2021"

			var team, opponent string
			if len(dateSegs) == 3 {
				game := dateSegs[2]
				if p := strings.SplitN(game, " at ", 2); len(p) == 2 {
					team, opponent = p[0], p[1]
				} else if p := strings.SplitN(game, " vs ", 2); len(p) == 2 {
					team, opponent = p[0], p[1]
				}
			}

			// quarter & time remaining
			qt := strings.SplitN(tipParts[1], ",", 2)
			quarter := qt[0]
			timeRem := strings.Fields(qt[1])[0]

			// result, shot type & distance
			rt := strings.Fields(tipParts[2]) // ["Made","2-pointer","..."|"Missed",...]
			made := rt[0] == "Made"
			shotType := rt[1]
			distance := mustAtoi(rt[len(rt)-2])

			// score & lead flag
			last := strings.Fields(tipParts[3])
			sc := strings.Split(last[len(last)-1], "-")
			teamScore, oppScore := mustAtoi(sc[0]), mustAtoi(sc[1])
			lead := teamScore > oppScore

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

			// ─────────────── 5) upsert / dedup  ────────────────
			if err := db.Clauses(clause.OnConflict{
				Columns: []clause.Column{ // MUST match the unique index order
					{Name: "player_id"},
					{Name: "season"},
					{Name: "date"},
					{Name: "qtr"},
					{Name: "time_remaining"},
					{Name: "top"},
					{Name: "left"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"player_name", "result", "shot_type", "distance_ft",
					"lead", "team_score", "opponent_team_score",
					"opponent", "team",
				}),
			}).Create(&shot).Error; err != nil && firstErr == nil {
				firstErr = err
			}
		})

		if firstErr != nil {
			return fmt.Errorf("DB upsert error for season %d: %w", season, firstErr)
		}
	}
	return nil
}

// extractCommentedShotChart returns the inner HTML of the comment block that
// contains <div id="div_shot-chart" …>. BR hides the SVG there for ad reasons.
func extractCommentedShotChart(htmlBytes []byte) string {
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
	parts := strings.Split(s, ":")
	return mustAtoi(strings.TrimSuffix(parts[1], "px"))
}