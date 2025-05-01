package services

import (
    "encoding/json"
    "fmt"
    "log"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils"
    "github.com/nprasad2077/NBA_Go/utils/metrics"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

func FetchAndStorePlayerAdvancedStats(db *gorm.DB, season int) error {
    metrics.DBOperationsTotal.WithLabelValues("fetch", "player_advanced").Inc()
    url := fmt.Sprintf("http://rest.nbaapi.com/api/PlayerDataAdvanced/query?season=%d&sortBy=Points&ascending=false&pageNumber=1&pageSize=1000", season)
    
    body, err := utils.GetJSON(url)
    if err != nil {
        return err
    }

    var stats []models.PlayerAdvancedStat
    if err := json.Unmarshal(body, &stats); err != nil {
        return err
    }

    for _, stat := range stats {
        // Ensure the season is included from the query param
        stat.Season = season

        err := db.Clauses(clause.OnConflict{
            Columns:   []clause.Column{{Name: "player_id"}, {Name: "season"}, {Name: "team"}},
            DoUpdates: clause.AssignmentColumns([]string{
                "external_id", "player_name", "position", "age", "games",
                "minutes_played", "per", "ts_percent", "three_par", "ftr",
                "offensive_rb_percent", "defensive_rb_percent", "total_rb_percent",
                "assist_percent", "steal_percent", "block_percent", "turnover_percent",
                "usage_percent", "offensive_ws", "defensive_ws", "win_shares",
                "win_shares_per", "offensive_box", "defensive_box", "box", "vorp",
            }),
        }).Create(&stat).Error
        
        metrics.DBOperationsTotal.WithLabelValues("store", "player_advanced").Inc()

        if err != nil {
            log.Printf("Failed to upsert stat for playerId %s (%s): %v", stat.PlayerID, stat.Team, err)
        }
    }

    return nil
}