// File: services/player_shot_chart_fetch_service.go
package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils"
	"github.com/nprasad2077/NBA_Go/utils/metrics"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FetchAndStoreShotChartForPlayer pulls the public NBA‑API JSON for a single
// player, derives the season from the shot date, and upserts into SQLite.
// Dupes are prevented by the unique index:
//
//   (player_id, season, date, qtr, time_remaining, top, left)
func FetchAndStoreShotChartForPlayer(db *gorm.DB, playerID string) error {
	metrics.DBOperationsTotal.WithLabelValues("fetch", "player_shot_chart").Inc()

	url := fmt.Sprintf("http://rest.nbaapi.com/api/ShotChartData/playerid/%s", playerID)
	body, err := utils.GetJSON(url)
	if err != nil {
		return err
	}

	var shots []models.PlayerShotChart
	if err := json.Unmarshal(body, &shots); err != nil {
		return err
	}

	for _, shot := range shots {
		// "Feb 11, 2023" → season 2023
		if t, err := time.Parse("Jan 2, 2006", shot.Date); err == nil {
			shot.Season = t.Year()
		}

		err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "player_id"},
				{Name: "season"},
				{Name: "date"},
				{Name: "qtr"},
				{Name: "time_remaining"},
				{Name: "top"},
				{Name: "left"},
			},
			// update mutable fields; leave the identity columns untouched
			DoUpdates: clause.AssignmentColumns([]string{
				"player_name", "result", "shot_type", "distance_ft",
				"lead", "team_score", "opponent_team_score",
				"opponent", "team",
			}),
		}).Create(&shot).Error

		metrics.DBOperationsTotal.WithLabelValues("store", "player_shot_chart").Inc()

		if err != nil {
			log.Printf("❌ upsert failed for %s on %s (%s %s): %v",
				shot.PlayerID, shot.Date, shot.Quarter, shot.TimeRemaining, err)
		}
	}
	return nil
}

// FetchAndStoreAllPlayerShotCharts enumerates every distinct player_id in your
// totals table, then calls the per‑player loader. It sleeps ~1.1 s between
// requests to stay well under public‑API rate limits.
func FetchAndStoreAllPlayerShotCharts(db *gorm.DB) error {
	var playerIDs []string

	// collect IDs (add other stats tables if needed)
	db.Model(&models.PlayerTotalStat{}).
		Distinct("player_id").
		Pluck("player_id", &playerIDs)

	for _, pid := range playerIDs {
		if err := FetchAndStoreShotChartForPlayer(db, pid); err != nil {
			log.Printf("Error importing shot chart for %s: %v", pid, err)
		}
		time.Sleep(1100 * time.Millisecond) // throttle
	}
	return nil
}