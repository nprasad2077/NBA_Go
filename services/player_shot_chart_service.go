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

// FetchAndStoreShotChartForPlayer fetches a single player's shot chart, parses
// each shot's date into a season, and upserts into DB.
func FetchAndStoreShotChartForPlayer(db *gorm.DB, playerId string) error {
    metrics.DBOperationsTotal.WithLabelValues("fetch", "player_shot_chart").Inc()

    url := fmt.Sprintf("http://rest.nbaapi.com/api/ShotChartData/playerid/%s", playerId)
    body, err := utils.GetJSON(url)
    if err != nil {
        return err
    }

    var shots []models.PlayerShotChart
    if err := json.Unmarshal(body, &shots); err != nil {
        return err
    }

    for _, shot := range shots {
        // parse date "Feb 11, 2023" → time.Time
        if t, err := time.Parse("Jan 2, 2006", shot.Date); err == nil {
            shot.Season = t.Year()
        }

        err := db.Clauses(clause.OnConflict{
            Columns:   []clause.Column{{Name: "player_id"}, {Name: "external_id"}},
            DoUpdates: clause.AssignmentColumns([]string{
                "player_name", "top", "left", "date", "qtr",
                "time_remaining", "result", "shot_type", "distance_ft",
                "lead", "team_score", "opponent_team_score", "opponent",
                "team", "season",
            }),
        }).Create(&shot).Error

        metrics.DBOperationsTotal.WithLabelValues("store", "player_shot_chart").Inc()

        if err != nil {
            log.Printf("Failed to upsert shot chart for %s id=%d: %v",
                shot.PlayerID, shot.ExternalID, err)
        }
    }
    return nil
}

// FetchAndStoreAllPlayerShotCharts loads every distinct playerId from your stats
// tables and invokes the per-player fetch.
func FetchAndStoreAllPlayerShotCharts(db *gorm.DB) error {
    var playerIds []string

    // gather from total stats (could also include advanced stats)
    db.Model(&models.PlayerTotalStat{}).
       Distinct("player_id").
       Pluck("player_id", &playerIds)

    for _, pid := range playerIds {
        if err := FetchAndStoreShotChartForPlayer(db, pid); err != nil {
            log.Printf("Error importing shot chart for %s: %v", pid, err)
        }
        // throttle
        time.Sleep(1100 * time.Millisecond)
    }
    return nil
}