package services

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func FetchAndStorePlayerTotalPlayoffsStats(db *gorm.DB, season int, isPlayoff bool) error {
	url := fmt.Sprintf(
		"http://rest.nbaapi.com/api/PlayerDataTotalsPlayoffs/query?season=%d&sortBy=PlayerName&ascending=true&pageNumber=1&pageSize=1000",
		season,
	)

	body, err := utils.GetJSON(url)
	if err != nil {
		return err
	}

	var stats []models.PlayerTotalStat
	if err := json.Unmarshal(body, &stats); err != nil {
		return err
	}

	for _, stat := range stats {
		stat.Season = season
		stat.IsPlayoff = isPlayoff

		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "player_id"}, {Name: "season"}, {Name: "team"}, {Name: "is_playoff"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"player_name", "position", "age", "games", "games_started",
				"minutes_pg", "field_goals", "field_attempts", "field_percent",
				"three_fg", "three_attempts", "three_percent", "two_fg", "two_attempts",
				"two_percent", "effect_fg_percent", "ft", "ft_attempts", "ft_percent",
				"offensive_rb", "defensive_rb", "total_rb", "assists", "steals",
				"blocks", "turnovers", "personal_fouls", "points",
			}),
		}).Create(&stat).Error

		if err != nil {
			log.Printf("Failed to upsert PlayerTotalStat for playerId %s: %v", stat.PlayerID, err)
		}
	}

	return nil
}