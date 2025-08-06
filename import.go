package main

import (
	"log"
	"time"

	"github.com/nprasad2077/NBA_Go/services"
	"github.com/nprasad2077/NBA_Go/utils"
	"gorm.io/gorm"
)

// importPlayerAdvanced fetches and stores advanced stats for seasons 2017–2025
func importPlayerAdvanced(db *gorm.DB) {
	for season := 2024; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, false); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		log.Printf("Advanced import for season: %d", season)
		time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1000 * time.Millisecond)
	}
}

// importPlayerAdvancedPlayoffs fetches and stores advanced stats for playoffs seasons 2023–2025
func importPlayerAdvancedPlayoffs(db *gorm.DB) {
	for season := 2024; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, true); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		log.Printf("Advanced Playoffs import for season: %d", season)
		time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1500 * time.Millisecond)
	}
}

// importPlayerTotalsScrape fetches & stores scraped regular-season total stats
func importPlayerTotalsScrape(db *gorm.DB) {
	for season := 2024; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, false); err != nil {
			log.Printf("scraped totals import failed for %d: %v", season, err)
		}
		log.Printf("Player Totals import for season: %d", season)
		time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1250 * time.Millisecond)
	}
}

// importPlayerPlayoffsScrape fetches & stores scraped playoff total stats
func importPlayerTotalsPlayoffsScrape(db *gorm.DB) {
	for season := 2024; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, true); err != nil {
			log.Printf("scraped playoffs import failed for %d: %v", season, err)
		}
		log.Printf("Player Playoffs Totals import for season: %d", season)
		time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1700 * time.Millisecond)
	}
}

// importGameSchedules fetches and stores game schedules
func importGameSchedules(db *gorm.DB) {
	// An NBA season typically runs from October to June
	months := []string{
		"october", "november", "december", "january",
		"february", "march", "april", "may", "june",

		// "february", "march", "april", "may", "june",
	}

	for season := 2025; season <= 2025; season++ {
		log.Printf("--- Starting Game Schedule Import for Season: %d ---", season)
		for _, month := range months {
			// The service will print a warning and skip if a month has no data (e.g. May/June for a season not yet finished)
			if err := services.FetchAndStoreGameSchedule(db, season, month); err != nil {
				// Log the error but continue to the next month/season
				log.Printf("Game schedule import failed for %s %d: %v", month, season, err)
			}
			log.Printf("Game schedule import for %s, %d complete.", month, season)
			// Respectful delay between requests
			time.Sleep(1100 * time.Millisecond)
			utils.SleepWithJitter(1800 * time.Millisecond)
		}
		log.Printf("--- Finished Game Schedule Import for Season: %d ---", season)
	}
}

// importBoxScores fetches and stores all box score data (line scores, player/team stats)
// for games within a recent date range.
func importBoxScores(db *gorm.DB) {
    // Define the date range for the 2023-2024 NBA season.
    // The regular season typically starts in October and playoffs end in June.
    from := time.Date(2022, time.October, 1, 0, 0, 0, 0, time.UTC)
    to := time.Date(2023, time.July, 1, 0, 0, 0, 0, time.UTC)

    log.Printf("--- Starting Box Score Data Import for the 2023-2024 Season ---")

    if err := services.FetchAndStoreBoxScoreDataForDateRange(db, from, to); err != nil {
        log.Fatalf("Box score import failed: %v", err)
    }

    log.Printf("--- Finished Box Score Data Import ---")
}

// importPlayerShotCharts fetches shot charts for a PREDEFINED list of players for a given range of seasons.
func importPlayerShotCharts(db *gorm.DB) {
	log.Println("--- Starting Player Shot Chart Import from Predefined List ---")

	// 1. Define the season range. Your service fetches from newest to oldest.
	startSeason := 2025
	endSeason := 2017 // Basketball-Reference has data going back this far.

	// 2. Define the static list of player IDs to import.
	// You can add or remove any Basketball-Reference player ID here.
	// Examples:
	// LeBron James:  "jamesle01"
	// Stephen Curry: "curryst01"
	// Nikola Jokić:  "jokicni01"
	// Luka Dončić:   "doncilu01"
	targetPlayerIDs := []string{
		"jamesle01",
		"curryst01",
		"jokicni01",
		"doncilu01",
		"hardeja01",
		"irvinky01",
		"duranke01",
		// "youngtr01",
		// "lillada01",
		// "gilgesh01",
		// "brunsja01",
		// "edwaran01",
		// "mitchdo01",
		// "bookede01",
		// "derozde01",

		// Add more player IDs here as needed
	}

	if len(targetPlayerIDs) == 0 {
		log.Println("⚠️ Player ID list is empty. Skipping shot chart import.")
		return
	}

	log.Printf("Found %d players in the predefined list. Starting shot chart import for seasons %d down to %d.", len(targetPlayerIDs), startSeason, endSeason)

	// 3. Loop through each player ID and fetch their shot chart data.
	for i, playerID := range targetPlayerIDs {
		log.Printf("(%d/%d) Importing shot charts for player: %s", i+1, len(targetPlayerIDs), playerID)

		err := services.FetchAndStoreShotChartScrapedForPlayer(db, playerID, startSeason, endSeason)
		if err != nil {
			// Log the error but continue with the next player.
			log.Printf("❌ Shot chart import failed for player %s: %v", playerID, err)
		}

		// 4. Be a good internet citizen. Pause between scraping each player.
		utils.SleepWithJitter(10000 * time.Millisecond)
	}

	log.Println("--- Finished Player Shot Chart Import from Predefined List ---")
}