package main

import (
	"log"
	"time"

	"gorm.io/gorm"
	"github.com/nprasad2077/NBA_Go/services"
	"github.com/nprasad2077/NBA_Go/utils"
)

// importPlayerAdvanced fetches and stores advanced stats for seasons 2017–2025
func importPlayerAdvanced(db *gorm.DB) {
	for season := 2025; season <= 2025; season++ {
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
	for season := 2025; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, true); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		log.Printf("Advanced Playoffs import for season: %d", season)
		time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1250 * time.Millisecond)
	}
}

// importPlayerTotalsScrape fetches & stores scraped regular-season total stats
func importPlayerTotalsScrape(db *gorm.DB) {
    for season := 2025; season <= 2025; season++ {
        if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, false); err != nil {
            log.Printf("scraped totals import failed for %d: %v", season, err)
        }
		log.Printf("Player Totals import for season: %d", season)
        time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1500 * time.Millisecond)
    }
}

// importPlayerPlayoffsScrape fetches & stores scraped playoff total stats
func importPlayerTotalsPlayoffsScrape(db *gorm.DB) {
    for season := 2025; season <= 2025; season++ {
        if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, true); err != nil {
            log.Printf("scraped playoffs import failed for %d: %v", season, err)
        }
		log.Printf("Player Playoffs Totals import for season: %d", season)
        time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1750 * time.Millisecond)
    }
}

// importGameSchedules fetches and stores game schedules
func importGameSchedules(db *gorm.DB) {
	// An NBA season typically runs from October to June
	months := []string{
		"october", "november", "december", "january",
		"february", "march", "april", "may", "june",
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
			utils.SleepWithJitter(1000 * time.Millisecond)
		}
		log.Printf("--- Finished Game Schedule Import for Season: %d ---", season)
	}
}


// importBoxScores fetches and stores all box score data (line scores, player/team stats)
// for games within a recent date range.
func importBoxScores(db *gorm.DB) {
	// Define the date range for the import.
	// This example fetches data for games in the last 90 days.
	to := time.Now()
	from := to.AddDate(0, -1, 1) // 0 years, -3 months, 0 days

	log.Printf("--- Starting Box Score Data Import from %s to %s ---", from.Format("2006-01-02"), to.Format("2006-01-02"))

	if err := services.FetchAndStoreBoxScoreDataForDateRange(db, from, to); err != nil {
		log.Fatalf("Box score import failed: %v", err)
	}

	log.Printf("--- Finished Box Score Data Import ---")
}




// importPlayerShotChart fetches shot-charts for every known player
// func importPlayerShotChart(db *gorm.DB) {
// 	const firstID = "hardeja01"
// 	log.Printf("▶️  importing shot chart for player %s…", firstID)
//     if err := services.FetchAndStoreShotChartForPlayer(db, firstID); err != nil {
//         log.Printf("shot chart import failed for %s: %v", firstID, err)
//     }
// 	// you can add more IDs here or just rely on the API endpoint after
// }
