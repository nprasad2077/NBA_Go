package models

import (
	"time"

	"gorm.io/gorm"
)

type PlayerAdvancedStat struct {
	ID					uint	`gorm:"primaryKey" swaggerignore:"true"`
	ExternalID          int     `json:"id"`
	PlayerID            string  `gorm:"not null;index:idx_player_season_team,unique" json:"playerId"`
	PlayerName          string  `json:"playerName"`
	Position            string  `json:"position"`
	Age                 int     `json:"age"`
	Games               int     `json:"games"`
	GamesStarted        int     `json:"gamesStarted"`
	MinutesPlayed       int     `json:"minutesPlayed"`
	PER                 float64 `json:"per"`
	TSPercent           float64 `json:"tsPercent"`
	ThreePAR            float64 `json:"threePAR"`
	FTR                 float64 `json:"ftr"`
	OffensiveRBPercent  float64 `json:"offensiveRBPercent"`
	DefensiveRBPercent  float64 `json:"defensiveRBPercent"`
	TotalRBPercent      float64 `json:"totalRBPercent"`
	AssistPercent       float64 `json:"assistPercent"`
	StealPercent        float64 `json:"stealPercent"`
	BlockPercent        float64 `json:"blockPercent"`
	TurnoverPercent     float64 `json:"turnoverPercent"`
	UsagePercent        float64 `json:"usagePercent"`
	OffensiveWS         float64 `json:"offensiveWS"`
	DefensiveWS         float64 `json:"defensiveWS"`
	WinShares           float64 `json:"winShares"`
	WinSharesPer        float64 `json:"winSharesPer"`
	OffensiveBox        float64 `json:"offensiveBox"`
	DefensiveBox        float64 `json:"defensiveBox"`
	Box                 float64 `json:"box"`
	VORP                float64 `json:"vorp"`
	Team                string  `gorm:"not null;index:idx_player_season_team,unique" json:"team"`
	Season              int     `gorm:"not null;index:idx_player_season_team,unique" json:"season"`
	IsPlayoff			bool	`gorm:"not null;default:false;index:idx_player_season_team,unique" json:"isPlayoff"`
	
	CreatedAt 			time.Time	`swaggerignore:"true"`
	UpdatedAt 			time.Time	`swaggerignore:"true"`
	DeletedAt 			gorm.DeletedAt	`gorm:"index" swaggerignore:"true"`
}