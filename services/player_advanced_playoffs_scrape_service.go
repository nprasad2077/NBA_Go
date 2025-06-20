// File: NBA_Go/services/player_advanced_scrape_service.go

package services

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/nprasad2077/NBA_Go/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	advancedURLFmt        = "https://www.basketball-reference.com/leagues/NBA_%d_advanced.html"
	advancedPlayoffURLFmt = "https://www.basketball-reference.com/playoffs/NBA_%d_advanced.html"
)

// urlForAdvSeason picks the correct URL based on isPlayoff.
func urlForAdvSeason(season int, isPlayoff bool) string {
	if isPlayoff {
		return fmt.Sprintf(advancedPlayoffURLFmt, season)
	}
	return fmt.Sprintf(advancedURLFmt, season)
}

// FetchAndStorePlayerAdvancedScrapedStats scrapes the advanced table (regular or playoffs)
// and batch upserts the data into the PlayerAdvancedStat model.
func FetchAndStorePlayerAdvancedScrapedStats(db *gorm.DB, season int, isPlayoff bool) error {
	url := urlForAdvSeason(season, isPlayoff)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return err
	}

	// 1) Determine parent wrapper and table selector. This logic is complex because
	// the table is often hidden inside an HTML comment.
	var parentDivSelector, tableSelector string
	if isPlayoff {
		parentDivSelector = "#all_advanced_stats"
		tableSelector = "table#advanced_stats"
	} else {
		parentDivSelector = "#all_advanced"
		tableSelector = "table#advanced"
	}

	table := doc.Find(parentDivSelector + " " + tableSelector)
	if table.Length() == 0 {
		commentSel := doc.
			Find(parentDivSelector).
			Contents().
			FilterFunction(func(i int, s *goquery.Selection) bool {
				return goquery.NodeName(s) == "#comment"
			})

		if commentSel.Length() == 0 {
			return fmt.Errorf("could not find advanced stats table (even inside comment) for season %d", season)
		}

		commentedHTML := commentSel.Nodes[0].FirstChild.Data
		innerDoc, err := goquery.NewDocumentFromReader(strings.NewReader(commentedHTML))
		if err != nil {
			return fmt.Errorf("failed to parse commented advanced HTML for season %d: %w", season, err)
		}
		table = innerDoc.Find(tableSelector)
		if table.Length() == 0 {
			return fmt.Errorf("could not find advanced stats table after un-commenting for season %d", season)
		}
	}

	// 2) Collect the data-stat keys in header order.
	var headers []string
	table.Find("thead tr th").Each(func(i int, th *goquery.Selection) {
		if stat, ok := th.Attr("data-stat"); ok && stat != "" {
			headers = append(headers, stat)
		}
	})
	headers = append(headers, "player-additional")

	// --- BATCHING LOGIC START ---
	// Create a slice to hold all the player stats parsed from the page.
	var statsToUpsert []models.PlayerAdvancedStat

	// 3) Iterate each row and collect player stats into the slice.
	table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
		if tr.HasClass("thead") {
			return // skip repeated header rows
		}
		cells := tr.Find("th, td")
		data := make(map[string]string, len(headers))
		var playerID string

		cells.Each(func(i int, cell *goquery.Selection) {
			key := headers[i]
			data[key] = strings.TrimSpace(cell.Text())
			if id, ok := cell.Attr("data-append-csv"); ok {
				playerID = id
			}
		})
		if playerID == "" {
			return // not a real data row
		}
		data["player-additional"] = playerID

		// 4) Determine ExternalID, PlayerName, Team.
		extID := mustAtoi(data["rk"])
		if extID == 0 {
			extID = mustAtoi(data["ranker"])
		}

		playerName := data["player"]
		if playerName == "" {
			playerName = data["name_display"]
		}

		teamID := data["team_id"]
		if teamID == "" {
			teamID = data["team_name_abbr"]
		}

		// 5) Games column is always "g" in advanced tables.
		g := mustAtoi(data["games"])
		if g == 0 {
			g = mustAtoi(data["g"])
		}

		// 6) Map into your GORM model.
		stat := models.PlayerAdvancedStat{
			ExternalID:         extID,
			PlayerID:           playerID,
			PlayerName:         playerName,
			Position:           data["pos"],
			Age:                mustAtoi(data["age"]),
			Games:              g,
			MinutesPlayed:      mustAtoi(data["mp"]),
			PER:                mustParseFloat(data["per"]),
			TSPercent:          mustParseFloat(data["ts_pct"]),
			ThreePAR:           mustParseFloat(data["fg3a_per_fga_pct"]),
			FTR:                mustParseFloat(data["fta_per_fga_pct"]),
			OffensiveRBPercent: mustParseFloat(data["orb_pct"]),
			DefensiveRBPercent: mustParseFloat(data["drb_pct"]),
			TotalRBPercent:     mustParseFloat(data["trb_pct"]),
			AssistPercent:      mustParseFloat(data["ast_pct"]),
			StealPercent:       mustParseFloat(data["stl_pct"]),
			BlockPercent:       mustParseFloat(data["blk_pct"]),
			TurnoverPercent:    mustParseFloat(data["tov_pct"]),
			UsagePercent:       mustParseFloat(data["usg_pct"]),
			OffensiveWS:        mustParseFloat(data["ows"]),
			DefensiveWS:        mustParseFloat(data["dws"]),
			WinShares:          mustParseFloat(data["ws"]),
			WinSharesPer:       mustParseFloat(data["ws_per_48"]),
			OffensiveBox:       mustParseFloat(data["obpm"]),
			DefensiveBox:       mustParseFloat(data["dbpm"]),
			Box:                mustParseFloat(data["bpm"]),
			VORP:               mustParseFloat(data["vorp"]),
			Team:               teamID,
			Season:             season,
			IsPlayoff:          isPlayoff,
		}

		// Add the parsed stat object to our slice.
		statsToUpsert = append(statsToUpsert, stat)
	})

	// 7) Perform the batch upsert operation after collecting all rows.
	if len(statsToUpsert) > 0 {
		log.Printf("Attempting to batch upsert %d advanced player stats for season %d...", len(statsToUpsert), season)

		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "player_id"},
				{Name: "season"},
				{Name: "team"},
				{Name: "is_playoff"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"external_id", "player_name", "position", "age", "games",
				"minutes_played", "per", "ts_percent", "three_par", "ftr",
				"offensive_rb_percent", "defensive_rb_percent", "total_rb_percent",
				"assist_percent", "steal_percent", "block_percent", "turnover_percent",
				"usage_percent", "offensive_ws", "defensive_ws", "win_shares",
				"win_shares_per", "offensive_box", "defensive_box", "box", "vorp",
			}),
		}).Create(&statsToUpsert).Error; err != nil {
			log.Printf("Failed to batch upsert advanced player stats: %v", err)
			return err
		}
		log.Printf("✅ Successfully batch upserted %d advanced records for season %d.", len(statsToUpsert), season)
	} else {
		log.Printf("No advanced player data found to import for season %d.", season)
	}
	// --- BATCHING LOGIC END ---

	return nil
}