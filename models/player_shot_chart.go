// NBA_Go/models/player_shot_chart.go
package models

import "gorm.io/gorm"

type PlayerShotChart struct {
    gorm.Model               `swaggerignore:"true"`
    ExternalID         int    `json:"id"`
    PlayerID           string `gorm:"not null;index:idx_shotchart_player_external,unique" json:"playerId"`
    PlayerName         string `json:"playerName"`
    Top                int    `json:"top"`
    Left               int    `json:"left"`
    Date               string `json:"date"`
    Quarter            string `gorm:"column:qtr" json:"qtr"`
    TimeRemaining      string `gorm:"column:time_remaining" json:"timeRemaining"`
    Result             bool   `json:"result"`
    ShotType           string `gorm:"column:shot_type" json:"shotType"`
    DistanceFt         int    `gorm:"column:distance_ft" json:"distanceFt"`
    Lead               bool   `json:"lead"`
    TeamScore          int    `gorm:"column:team_score" json:"teamScore"`
    OpponentTeamScore  int    `gorm:"column:opponent_team_score" json:"opponentTeamScore"`
    Opponent           string `json:"opponent"`
    Team               string `gorm:"not null" json:"team"`
    Season             int    `gorm:"not null" json:"season"`
}