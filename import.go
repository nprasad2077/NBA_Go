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
	for season := 2017; season <= 2022; season++ {
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
	for season := 2017; season <= 2022; season++ {
		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, true); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		log.Printf("Advanced Playoffs import for season: %d", season)
		time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1250 * time.Millisecond)
	}
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

// importPlayerTotalsScrape fetches & stores scraped regular-season total stats
func importPlayerTotalsScrape(db *gorm.DB) {
    for season := 2017; season <= 2022; season++ {
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
    for season := 2017; season <= 2022; season++ {
        if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, true); err != nil {
            log.Printf("scraped playoffs import failed for %d: %v", season, err)
        }
		log.Printf("Player Playoffs Totals import for season: %d", season)
        time.Sleep(1100 * time.Millisecond)
		utils.SleepWithJitter(1750 * time.Millisecond)
    }
}