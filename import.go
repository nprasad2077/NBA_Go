package main

import (
	"log"
	"time"

	"gorm.io/gorm"
	"github.com/nprasad2077/NBA_Go/services"
)

// importPlayerAdvanced fetches and stores advanced stats for seasons 2023–2025
func importPlayerAdvanced(db *gorm.DB) {
	for season := 2023; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerAdvancedStats(db, season, false); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		time.Sleep(1100 * time.Millisecond)
	}
}

// importPlayerAdvancedPlayoffs fetches and stores advanced stats for playoffs seasons 2023–2025

func importPlayerAdvancedPlayoffs (db *gorm.DB) {
	for season := 2023; season <= 2024; season++ {
		if err := services.FetchAndStorePlayerAdvancedPlayoffsStats(db, season, true); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		time.Sleep(1100 * time.Millisecond)
	}
}

// importPlayerTotals fetches and stores regular-season total stats for seasons 2023–2025
func importPlayerTotals(db *gorm.DB) {
	for season := 2023; season <= 2025; season++ {
		if err := services.FetchAndStorePlayerTotalStats(db, season, false); err != nil {
			log.Printf("totals import failed for %d: %v", season, err)
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

// importPlayerPlayoffs fetches and stores playoff total stats for seasons 2023–2024
func importPlayerPlayoffs(db *gorm.DB) {
	for season := 2023; season <= 2024; season++ {
		if err := services.FetchAndStorePlayerTotalPlayoffsStats(db, season, true); err != nil {
			log.Printf("playoffs import failed for %d: %v", season, err)
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

// importPlayerShotChart fetches shot-charts for every known player
func importPlayerShotChart(db *gorm.DB) {
	const firstID = "hardeja01"
	log.Printf("▶️  importing shot chart for player %s…", firstID)
    if err := services.FetchAndStoreShotChartForPlayer(db, firstID); err != nil {
        log.Printf("shot chart import failed for %s: %v", firstID, err)
    }
	// you can add more IDs here or just rely on the API endpoint after
}
