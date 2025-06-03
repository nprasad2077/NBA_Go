package models

import (
	"time"

	"gorm.io/gorm"
)

type PlayerTotalStat struct {
	ID				uint	`gorm:"primaryKey" swaggerignore:"true"`

	ExternalID      int     `json:"id"`
	PlayerID        string  `gorm:"not null;uniqueIndex:idx_total_player_season_team" json:"playerId"`
	PlayerName      string  `json:"playerName"`
	Position        string  `json:"position"`
	Age             int     `json:"age"`
	Games           int     `json:"games"`
	GamesStarted    int     `json:"gamesStarted"`
	MinutesPG       float64     `json:"minutesPg"`
	FieldGoals      int     `json:"fieldGoals"`
	FieldAttempts   int     `json:"fieldAttempts"`
	FieldPercent    float64 `json:"fieldPercent"`
	ThreeFG         int     `json:"threeFg"`
	ThreeAttempts   int     `json:"threeAttempts"`
	ThreePercent    float64 `json:"threePercent"`
	TwoFG           int     `json:"twoFg"`
	TwoAttempts     int     `json:"twoAttempts"`
	TwoPercent      float64 `json:"twoPercent"`
	EffectFGPercent float64 `json:"effectFgPercent"`
	FT              int     `json:"ft"`
	FTAttempts      int     `json:"ftAttempts"`
	FTPercent       float64 `json:"ftPercent"`
	OffensiveRB     int     `json:"offensiveRb"`
	DefensiveRB     int     `json:"defensiveRb"`
	TotalRB         int     `json:"totalRb"`
	Assists         int     `json:"assists"`
	Steals          int     `json:"steals"`
	Blocks          int     `json:"blocks"`
	Turnovers       int     `json:"turnovers"`
	PersonalFouls   int     `json:"personalFouls"`
	Points          int     `json:"points"`
	Team            string  `gorm:"not null;uniqueIndex:idx_total_player_season_team" json:"team"`
	Season          int     `gorm:"not null;uniqueIndex:idx_total_player_season_team" json:"season"`
	IsPlayoff		bool	`gorm:"not null;default:false;uniqueIndex:idx_total_player_season_team" json:"isPlayoff"`
	
	CreatedAt 		time.Time	`swaggerignore:"true"`
	UpdatedAt 		time.Time	`swaggerignore:"true"`
	DeletedAt 		gorm.DeletedAt	`gorm:"index" swaggerignore:"true"`
}