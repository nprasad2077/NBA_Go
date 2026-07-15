package main

import (
	"log"
	"time"
	"fmt"

	"github.com/nprasad2077/NBA_Go/services"
	"github.com/nprasad2077/NBA_Go/utils"
	"gorm.io/gorm"
)

// importPlayerAdvanced fetches and stores advanced stats for seasons
func importPlayerAdvanced(db *gorm.DB) {
	for season := 2026; season <= 2026; season++ {
		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, false); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		log.Printf("Advanced import for season: %d", season)
		time.Sleep(2000 * time.Millisecond)
		utils.SleepWithJitter(1000 * time.Millisecond)
	}
}

// importPlayerAdvancedPlayoffs fetches and stores advanced stats for playoffs seasons
func importPlayerAdvancedPlayoffs(db *gorm.DB) {
	for season := 2026; season <= 2026; season++ {
		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, true); err != nil {
			log.Printf("advanced import failed for %d: %v", season, err)
		}
		log.Printf("Advanced Playoffs import for season: %d", season)
		time.Sleep(2000 * time.Millisecond)
		utils.SleepWithJitter(1500 * time.Millisecond)
	}
}

// importPlayerTotalsScrape fetches & stores scraped regular-season total stats
func importPlayerTotalsScrape(db *gorm.DB) {
	for season := 2026; season <= 2026; season++ {
		if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, false); err != nil {
			log.Printf("scraped totals import failed for %d: %v", season, err)
		}
		log.Printf("Player Totals import for season: %d", season)
		time.Sleep(2000 * time.Millisecond)
		utils.SleepWithJitter(1250 * time.Millisecond)
	}
}

// importPlayerPlayoffsScrape fetches & stores scraped playoff total stats
func importPlayerTotalsPlayoffsScrape(db *gorm.DB) {
	for season := 2026; season <= 2026; season++ {
		if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, true); err != nil {
			log.Printf("scraped playoffs import failed for %d: %v", season, err)
		}
		log.Printf("Player Playoffs Totals import for season: %d", season)
		time.Sleep(2000 * time.Millisecond)
		utils.SleepWithJitter(1700 * time.Millisecond)
	}
}

// importGameSchedules fetches and stores game schedules
func importGameSchedules(db *gorm.DB) {
	months := []string{"june"}

	for season := 2010; season <= 2010; season++ {
		log.Printf("--- Starting Game Schedule Import for Season: %d ---", season)
		for _, month := range months {
			if err := services.FetchAndStoreGameSchedule(db, season, month); err != nil {
				log.Printf("Game schedule import failed for %s %d: %v", month, season, err)
			}
			log.Printf("Game schedule import for %s, %d complete.", month, season)
			time.Sleep(2500 * time.Millisecond)
			utils.SleepWithJitter(1800 * time.Millisecond)
		}
		log.Printf("--- Finished Game Schedule Import for Season: %d ---", season)
	}
}

// importBoxScores fetches and stores all box score data (line scores, player/team stats)
// for games within a recent date range.
func importBoxScores(db *gorm.DB) {
	from := time.Date(2010, time.June, 7, 0, 0, 0, 0, time.UTC)
	to := time.Date(2010, time.June, 27, 5, 30, 0, 0, time.UTC)

	dateRangeComment := fmt.Sprintf("--- Starting Box Score Data Import for games between %s and %s ---",
		from.Format("January 2, 2006"),
		to.Format("January 2, 2006"))

	log.Println(dateRangeComment)

	if err := services.FetchAndStoreBoxScoreDataForDateRange(db, from, to); err != nil {
		log.Fatalf("Box score import failed: %v", err)
	}

	log.Println("--- Finished Box Score Data Import ---")
}

// importPlayerShotCharts fetches shot charts for a PREDEFINED list of players for a given range of seasons.
func importPlayerShotCharts(db *gorm.DB) {
	log.Println("--- Starting Player Shot Chart Import from Predefined List ---")

	startSeason := 2026
	endSeason := 2017

	targetPlayerIDs := []string{
		"jamesle01",
		"curryst01",
		"jokicni01",
		"doncilu01",
		"hardeja01",
		"irvinky01",
		"duranke01",
		"youngtr01",
		"gilgesh01",
		"brunsja01",
		"edwaran01",
		"mitchdo01",
		"derozde01",
		"westbru01",
		"butleji01",
		"davisan02",
		"leonaka01",
		"tatumja01",
	}

	if len(targetPlayerIDs) == 0 {
		log.Println("⚠️ Player ID list is empty. Skipping shot chart import.")
		return
	}

	log.Printf("Found %d players in the predefined list. Starting shot chart import for seasons %d down to %d.", len(targetPlayerIDs), startSeason, endSeason)

	for i, playerID := range targetPlayerIDs {
		log.Printf("(%d/%d) Importing shot charts for player: %s", i+1, len(targetPlayerIDs), playerID)

		err := services.FetchAndStoreShotChartScrapedForPlayer(db, playerID, startSeason, endSeason)
		if err != nil {
			log.Printf("❌ Shot chart import failed for player %s: %v", playerID, err)
		}

		utils.SleepWithJitter(10000 * time.Millisecond)
	}

	log.Println("--- Finished Player Shot Chart Import from Predefined List ---")
}

// importMarkPlayoffGames marks games as playoff using the dedicated Basketball Reference playoff schedule.
func importMarkPlayoffGames(db *gorm.DB) {
	for season := 2010; season <= 2010; season++ {
		if err := services.FetchAndMarkPlayoffGames(db, season+1); err != nil {
			log.Printf("playoff marking failed for %d: %v", season, err)
		}
		log.Printf("Playoff games marked for season: %d", season)
	}
}
