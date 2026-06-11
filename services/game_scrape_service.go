// File: services/game_scrape_service.go
package services

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/nprasad2077/NBA_Go/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const gameScheduleURLFmt = "https://www.basketball-reference.com/leagues/NBA_%d_games-%s.html"
const playoffScheduleURLFmt = "https://www.basketball-reference.com/playoffs/NBA_%d_games.html"

// FetchAndMarkPlayoffGames scrapes the dedicated playoff schedule page and
// batch-updates all matching games in the DB to set IsPlayoff=true.
// This is a post-processing step that does not modify the normal import flow.
func FetchAndMarkPlayoffGames(db *gorm.DB, season int) error {
	url := fmt.Sprintf(playoffScheduleURLFmt, season)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch playoff schedule for %d: %w", season, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("playoff schedule returned status %s for season %d", resp.Status, season)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse playoff schedule HTML: %w", err)
	}

	var gameIDs []string
	doc.Find(`td[data-stat="box_score_text"] a`).Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			parts := strings.Split(href, "/")
			fileName := parts[len(parts)-1]
			gameID := strings.TrimSuffix(fileName, ".html")
			if gameID != "" {
				gameIDs = append(gameIDs, gameID)
			}
		}
	})

	if len(gameIDs) == 0 {
		log.Printf("No playoff game IDs found for season %d.", season)
		return nil
	}

	if err := db.Model(&models.Game{}).Where("game_id IN ?", gameIDs).Update("is_playoff", true).Error; err != nil {
		return fmt.Errorf("failed to mark playoff games: %w", err)
	}

	log.Printf("✅ Marked %d games as playoff for season %d.", len(gameIDs), season)
	return nil
}

// FetchAndStoreGameSchedule scrapes the game schedule for a given season and month.
// The month should be the full lowercase name, e.g., "october", "november".
// If db is nil, it will perform a "dry run" and print the parsed data to the console.
func FetchAndStoreGameSchedule(db *gorm.DB, season int, month string) error {
	url := fmt.Sprintf(gameScheduleURLFmt, season, month)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch schedule for %s %d: %w", month, season, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️  Skipping schedule for %s %d (Status: %s)", month, season, resp.Status)
		return nil // Not a fatal error, just no data for this month.
	}

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body for %s %d: %w", month, season, err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return fmt.Errorf("failed to parse HTML for %s %d: %w", month, season, err)
	}

	table := doc.Find("table#schedule")
	if table.Length() == 0 {
		// Sometimes the content is commented out
		commentNode := doc.Find("#all_schedule").Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
			return goquery.NodeName(s) == "#comment"
		})
		if commentNode.Length() > 0 {
			commentedHTML := commentNode.Nodes[0].FirstChild.Data
			innerDoc, err := goquery.NewDocumentFromReader(strings.NewReader(commentedHTML))
			if err != nil {
				return fmt.Errorf("failed to parse commented schedule HTML: %w", err)
			}
			table = innerDoc.Find("table#schedule")
		}
	}

	if table.Length() == 0 {
		log.Printf("No schedule table found for %s %d.", month, season)
		return nil
	}

	var gamesToUpsert []models.Game
	table.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		// Skip table header rows that are sometimes repeated in the body
		if row.Find("th.poptip").Length() > 1 {
			return
		}

		// Skip rows that don't represent games (e.g., placeholder rows)
		if row.Find(`[data-stat="visitor_team_name"]`).Text() == "" {
			return
		}

		var game models.Game
		var gameID string

		// Extract GameID from the box score link, which is the most reliable unique key
		boxScoreCell := row.Find(`td[data-stat="box_score_text"] a`)
		if href, exists := boxScoreCell.Attr("href"); exists {
			parts := strings.Split(href, "/")
			fileName := parts[len(parts)-1]
			gameID = strings.TrimSuffix(fileName, ".html")
		}

		// If there's no box score link, it's likely a future game, we can skip it or handle differently
		if gameID == "" {
			return
		}
		game.GameID = gameID

		// Get the date part from the 'csk' attribute for accuracy
		dateCsk, _ := row.Find(`th[data-stat="date_game"]`).Attr("csk")
		var datePart string
		if len(dateCsk) >= 8 {
			datePart = dateCsk[:8]
		} else {
			log.Printf("Could not parse date from invalid 'csk' attribute: %s. Skipping.", dateCsk)
			return
		}

		// Get the start time string
		startTimeET := row.Find(`td[data-stat="game_start_time"]`).Text()
		if startTimeET == "" {
			// Skip games without a start time, as they can't be parsed accurately
			log.Printf("Could not find start time for game %s. Skipping.", gameID)
			return
		}

		// Load the US/Eastern timezone to correctly handle ET/EST/EDT
		eastern, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Printf("FATAL: Could not load America/New_York timezone: %v", err)
			// This is a system-level error, so we stop the row processing here.
			// The function will continue and process any games already parsed.
			return
		}

		// Combine date and time and parse together in the correct timezone.
		// The layout "3:04p" handles times like "7:30p".
		fullDateTimeString := datePart + startTimeET
		layout := "200601023:04p"
		gameDate, err := time.ParseInLocation(layout, fullDateTimeString, eastern)
		if err != nil {
			log.Printf("Could not parse combined date-time for game %s (value: '%s'): %v. Skipping.", gameID, fullDateTimeString, err)
			return
		}
		game.Date = gameDate
		game.StartTimeET = startTimeET // Keep the original string as well

		game.VisitorTeam = row.Find(`td[data-stat="visitor_team_name"] a`).Text()
		game.VisitorPTS = mustAtoi(row.Find(`td[data-stat="visitor_pts"]`).Text())
		game.HomeTeam = row.Find(`td[data-stat="home_team_name"] a`).Text()
		game.HomePTS = mustAtoi(row.Find(`td[data-stat="home_pts"]`).Text())
		game.BoxScoreURL, _ = boxScoreCell.Attr("href")
		game.GameDuration = row.Find(`td[data-stat="game_duration"]`).Text()
		game.Arena = row.Find(`td[data-stat="arena_name"]`).Text()
		game.IsPlayoff = strings.Contains(row.Find(`td[data-stat="game_remarks"]`).Text(), "Playoffs")

		gamesToUpsert = append(gamesToUpsert, game)
	})

	if len(gamesToUpsert) > 0 {
		// If the db connection is nil, we're in test/debug mode. Print to console.
		if db == nil {
			log.Println("--- RUNNING IN DRY-RUN MODE ---")
			for _, game := range gamesToUpsert {
				// Use %+v to print the struct with field names for clarity
				log.Printf("Game Data: %+v\n", game)
			}
			log.Printf("--- WOULD INSERT %d RECORDS ---", len(gamesToUpsert))
			return nil // End execution for dry-run
		}

		log.Printf("Attempting to batch upsert %d games for %s %d...", len(gamesToUpsert), month, season)

		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "game_id"}},
			DoUpdates: clause.AssignmentColumns(allGameColumns()),
		}).Create(&gamesToUpsert).Error; err != nil {
			log.Printf("Failed to batch upsert games: %v", err)
			return err
		}
		log.Printf("✅ Successfully batch upserted %d game records for %s %d.", len(gamesToUpsert), month, season)
	} else {
		log.Printf("No game data found to import for %s %d.", month, season)
	}

	return nil
}

// allGameColumns returns a list of all column names in the Game model for the upsert operation.
// This ensures that if a record exists, all its fields are updated with the new data.
func allGameColumns() []string {
	return []string{
		"date", "is_playoff", "start_time_et", "arena", "visitor_team",
		"visitor_pts", "home_team", "home_pts", "game_duration", "box_score_url",
		"updated_at",
	}
}
