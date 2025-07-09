package models

import (
	"gorm.io/gorm"
	"time"
)

type Game struct {
	ID           uint      `gorm:"primaryKey" swaggerignore:"true"`
	GameID       string    `gorm:"not null;uniqueIndex" json:"gameId"` // The field you want to use
	Date         time.Time `gorm:"not null;index" json:"date"`
	IsPlayoff    bool      `gorm:"not null;default:false;index" json:"isPlayoff"`
	StartTimeET  string    `json:"startTimeET"`
	Arena        string    `json:"arena"`
	VisitorTeam  string    `gorm:"not null" json:"visitorTeam"`
	VisitorPTS   int       `json:"visitorPts"`
	HomeTeam     string    `gorm:"not null" json:"homeTeam"`
	HomePTS      int       `json:"homePts"`
	GameDuration string    `json:"gameDuration"`
	BoxScoreURL  string    `json:"boxScoreUrl"`

	// Associations (Updated with explicit foreign key tags)
	LineScores           []LineScore           `gorm:"foreignKey:GameID;references:GameID" json:"lineScores"`
	PlayerGameBasicStats []PlayerGameBasicStat `gorm:"foreignKey:GameID;references:GameID" json:"playerGameBasicStats"`
	PlayerGameAdvStats   []PlayerGameAdvStat   `gorm:"foreignKey:GameID;references:GameID" json:"playerGameAdvStats"`
	TeamGameBasicStats   []TeamGameBasicStat   `gorm:"foreignKey:GameID;references:GameID" json:"teamGameBasicStats"`
	TeamGameAdvStats     []TeamGameAdvStat     `gorm:"foreignKey:GameID;references:GameID" json:"teamGameAdvStats"`

	CreatedAt time.Time      `swaggerignore:"true"`
	UpdatedAt time.Time      `swaggerignore:"true"`
	DeletedAt gorm.DeletedAt `gorm:"index" swaggerignore:"true"`
}

// LineScore holds the scoring for each team by quarter for a specific game.
type LineScore struct {
	ID     uint   `gorm:"primaryKey" swaggerignore:"true"`
	GameID string `gorm:"not null;uniqueIndex:idx_line_score,priority:1" json:"gameId"`
	Team   string `gorm:"not null;uniqueIndex:idx_line_score,priority:2" json:"team"`
	Q1     int    `json:"q1"`
	Q2     int    `json:"q2"`
	Q3     int    `json:"q3"`
	Q4     int    `json:"q4"`
	// Overtime scores; will be 0 if the game did not go to the respective OT period.
	OT1   int `json:"ot1"`
	OT2   int `json:"ot2"`
	OT3   int `json:"ot3"`
	Total int `json:"total"`

	CreatedAt time.Time      `swaggerignore:"true"`
	UpdatedAt time.Time      `swaggerignore:"true"`
	DeletedAt gorm.DeletedAt `gorm:"index" swaggerignore:"true"`
}

// PlayerGameBasicStat contains the basic box score statistics for a single player in a single game.
type PlayerGameBasicStat struct {
	ID            uint    `gorm:"primaryKey" swaggerignore:"true"`
	GameID        string  `gorm:"not null;uniqueIndex:idx_player_game_basic" json:"gameId"`
	PlayerID      string  `gorm:"not null;uniqueIndex:idx_player_game_basic" json:"playerId"`
	PlayerName    string  `json:"playerName"`
	Team          string  `gorm:"not null" json:"team"`
	Status        string  `json:"status"` // e.g., "Starter", "Bench", "Did Not Play"
	MP            string  `json:"mp"`     // Minutes Played, e.g., "38:07"
	FG            int     `json:"fg"`
	FGA           int     `json:"fga"`
	FGPercent     float64 `json:"fgPercent"`
	ThreeP        int     `json:"threeP"`
	ThreePA       int     `json:"threePa"`
	ThreePPercent float64 `json:"threePPercent"`
	FT            int     `json:"ft"`
	FTA           int     `json:"fta"`
	FTPercent     float64 `json:"ftPercent"`
	ORB           int     `json:"orb"`
	DRB           int     `json:"drb"`
	TRB           int     `json:"trb"`
	AST           int     `json:"ast"`
	STL           int     `json:"stl"`
	BLK           int     `json:"blk"`
	TOV           int     `json:"tov"`
	PF            int     `json:"pf"`
	PTS           int     `json:"pts"`
	GmSc          float64 `json:"gmSc"` // Game Score
	PlusMinus     int     `json:"plusMinus"`

	CreatedAt time.Time      `swaggerignore:"true"`
	UpdatedAt time.Time      `swaggerignore:"true"`
	DeletedAt gorm.DeletedAt `gorm:"index" swaggerignore:"true"`
}

// PlayerGameAdvStat contains the advanced box score statistics for a single player in a single game.
type PlayerGameAdvStat struct {
	ID         uint    `gorm:"primaryKey" swaggerignore:"true"`
	GameID     string  `gorm:"not null;uniqueIndex:idx_player_game_adv" json:"gameId"`
	PlayerID   string  `gorm:"not null;uniqueIndex:idx_player_game_adv" json:"playerId"`
	PlayerName string  `json:"playerName"`
	Team       string  `gorm:"not null" json:"team"`
	MP         string  `json:"mp"` // Minutes Played
	TSPercent  float64 `json:"tsPercent"`
	EFGPercent float64 `json:"efgPercent"`
	ThreePAr   float64 `json:"threePAr"`
	FTr        float64 `json:"fTr"`
	ORBPercent float64 `json:"orbPercent"`
	DRBPercent float64 `json:"drbPercent"`
	TRBPercent float64 `json:"trbPercent"`
	ASTPercent float64 `json:"astPercent"`
	STLPercent float64 `json:"stlPercent"`
	BLKPercent float64 `json:"blkPercent"`
	TOVPercent float64 `json:"tovPercent"`
	USGPercent float64 `json:"usgPercent"`
	ORtg       int     `json:"oRtg"`
	DRtg       int     `json:"dRtg"`
	BPM        float64 `json:"bpm"` // Box Plus/Minus

	CreatedAt time.Time      `swaggerignore:"true"`
	UpdatedAt time.Time      `swaggerignore:"true"`
	DeletedAt gorm.DeletedAt `gorm:"index" swaggerignore:"true"`
}

// TeamGameBasicStat holds the total basic stats for a team in a single game.
type TeamGameBasicStat struct {
	ID            uint    `gorm:"primaryKey" swaggerignore:"true"`
	GameID        string  `gorm:"not null;uniqueIndex:idx_team_game_basic" json:"gameId"`
	Team          string  `gorm:"not null;uniqueIndex:idx_team_game_basic" json:"team"`
	MP            int     `json:"mp"` // Total minutes, usually 240 for a regulation game
	FG            int     `json:"fg"`
	FGA           int     `json:"fga"`
	FGPercent     float64 `json:"fgPercent"`
	ThreeP        int     `json:"threeP"`
	ThreePA       int     `json:"threePa"`
	ThreePPercent float64 `json:"threePPercent"`
	FT            int     `json:"ft"`
	FTA           int     `json:"fta"`
	FTPercent     float64 `json:"ftPercent"`
	ORB           int     `json:"orb"`
	DRB           int     `json:"drb"`
	TRB           int     `json:"trb"`
	AST           int     `json:"ast"`
	STL           int     `json:"stl"`
	BLK           int     `json:"blk"`
	TOV           int     `json:"tov"`
	PF            int     `json:"pf"`
	PTS           int     `json:"pts"`

	CreatedAt time.Time      `swaggerignore:"true"`
	UpdatedAt time.Time      `swaggerignore:"true"`
	DeletedAt gorm.DeletedAt `gorm:"index" swaggerignore:"true"`
}

// TeamGameAdvStat holds the total advanced stats for a team in a single game.
type TeamGameAdvStat struct {
	ID         uint    `gorm:"primaryKey" swaggerignore:"true"`
	GameID     string  `gorm:"not null;uniqueIndex:idx_team_game_adv" json:"gameId"`
	Team       string  `gorm:"not null;uniqueIndex:idx_team_game_adv" json:"team"`
	MP         int     `json:"mp"`
	TSPercent  float64 `json:"tsPercent"`
	EFGPercent float64 `json:"efgPercent"`
	ThreePAr   float64 `json:"threePAr"`
	FTr        float64 `json:"fTr"`
	ORBPercent float64 `json:"orbPercent"`
	DRBPercent float64 `json:"drbPercent"`
	TRBPercent float64 `json:"trbPercent"`
	ASTPercent float64 `json:"astPercent"`
	STLPercent float64 `json:"stlPercent"`
	BLKPercent float64 `json:"blkPercent"`
	TOVPercent float64 `json:"tovPercent"`
	USGPercent float64 `json:"usgPercent"` // Will be ~100.0 for a team
	ORtg       float64 `json:"oRtg"`
	DRtg       float64 `json:"dRtg"`

	CreatedAt time.Time      `swaggerignore:"true"`
	UpdatedAt time.Time      `swaggerignore:"true"`
	DeletedAt gorm.DeletedAt `gorm:"index" swaggerignore:"true"`
}
