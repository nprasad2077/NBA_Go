// models/player_shot_chart.go
package models

import "gorm.io/gorm"

type PlayerShotChart struct {
    // Auto‑increment primary key — works in SQLite and any other DB.
    ID uint `gorm:"primaryKey" json:"id"`

    // ──────────  "identity" columns (the dedup key)  ──────────
    PlayerID       string `gorm:"not null;uniqueIndex:idx_shot_identity,priority:1" json:"playerId"`
    Season         int    `gorm:"not null;uniqueIndex:idx_shot_identity,priority:2" json:"season"`
    Date           string `gorm:"not null;uniqueIndex:idx_shot_identity,priority:3" json:"date"`
    Quarter        string `gorm:"column:qtr;uniqueIndex:idx_shot_identity,priority:4" json:"qtr"`
    TimeRemaining  string `gorm:"column:time_remaining;uniqueIndex:idx_shot_identity,priority:5" json:"timeRemaining"`
    Top            int    `gorm:"uniqueIndex:idx_shot_identity,priority:6" json:"top"`
    Left           int    `gorm:"uniqueIndex:idx_shot_identity,priority:7" json:"left"`

    // ──────────  the rest of the payload  ──────────
    PlayerName        string `json:"playerName"`
    Result            bool   `json:"result"`
    ShotType          string `gorm:"column:shot_type" json:"shotType"`
    DistanceFt        int    `gorm:"column:distance_ft" json:"distanceFt"`
    Lead              bool   `json:"lead"`
    TeamScore         int    `gorm:"column:team_score" json:"teamScore"`
    OpponentTeamScore int    `gorm:"column:opponent_team_score" json:"opponentTeamScore"`
    Opponent          string `json:"opponent"`
    Team              string `gorm:"not null" json:"team"`

    gorm.Model        `swaggerignore:"true"` // keeps CreatedAt/UpdatedAt/DeletedAt
}