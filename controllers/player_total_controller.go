package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/services"
	"gorm.io/gorm"
)

// --- MODIFICATION START ---
// A DTO is created to control the JSON output.
// It omits fields like ID, ExternalID, CreatedAt, UpdatedAt, and DeletedAt.
type PlayerTotalStatDTO struct {
	PlayerID        string  `json:"playerId"`
	PlayerName      string  `json:"playerName"`
	Position        string  `json:"position"`
	Age             int     `json:"age"`
	Games           int     `json:"games"`
	GamesStarted    int     `json:"gamesStarted"`
	MinutesPG       float64 `json:"minutesPg"`
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
	Team            string  `json:"team"`
	Season          int     `json:"season"`
	IsPlayoff       bool    `json:"isPlayoff"`
}

// A dedicated response struct is created for cleaner, type-safe code.
type PlayerTotalsResponse struct {
	Data       []PlayerTotalStatDTO `json:"data"`
	Pagination struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Pages    int64 `json:"pages"`
	} `json:"pagination"`
}
// --- MODIFICATION END ---


var totalSortMap = map[string]string{
	"playerId":        "player_id",
	"playerName":      "player_name",
	"position":        "position",
	"age":             "age",
	"games":           "games",
	"gamesStarted":    "games_started",
	"minutesPg":       "minutes_pg",
	"fieldGoals":      "field_goals",
	"fieldAttempts":   "field_attempts",
	"fieldPercent":    "field_percent",
	"threeFg":         "three_fg",
	"threeAttempts":   "three_attempts",
	"threePercent":    "three_percent",
	"twoFg":           "two_fg",
	"twoAttempts":     "two_attempts",
	"twoPercent":      "two_percent",
	"effectFgPercent": "effect_fg_percent",
	"ft":              "ft",
	"ftAttempts":      "ft_attempts",
	"ftPercent":       "ft_percent",
	"offensiveRb":     "offensive_rb",
	"defensiveRb":     "defensive_rb",
	"totalRb":         "total_rb",
	"assists":         "assists",
	"steals":          "steals",
	"blocks":          "blocks",
	"turnovers":       "turnovers",
	"personalFouls":   "personal_fouls",
	"points":          "points",
	"team":            "team",
	"season":          "season",
}

// ScrapePlayerTotalStats godoc
// @ignore
// @Summary      Scrape player total stats from BR website
// @Tags         PlayerTotals
// @Param        season    query  int  true  "Season (e.g. 2025)"
// @Param        isPlayoff query  bool false "Whether playoffs?"
// @Success      200       {object} map[string]string
// @Failure      400,500   {object} map[string]string
// //@Router       /api/playertotals/scrape [get]
func ScrapePlayerTotalStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		season := c.QueryInt("season", 0)
		if season == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "season is required"})
		}
		isPlayoff := c.QueryBool("isPlayoff", false)

		if err := services.FetchAndStorePlayerTotalScrapedStats(db, season, isPlayoff); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "scrapestore complete"})
	}
}

// GetPlayerTotalStats godoc
// //@Security ApiKeyAuth
// @Summary Get player total stats
// @Description Filter and paginate player totals
// @Tags PlayerTotals
// @x-order 1
// @Group Player Stats
// @Accept   json
// @Produce  json
// @Param season query int false "Season (e.g. 2000)"
// @Param team query string false "Team abbreviation (e.g. LAL)"
// @Param playerId query string false "Player ID (e.g. greenac01)"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param sortBy query string false "Field to sort by (e.g. points, assists)"
// @Param ascending query bool false "Sort ascending (default false)"
// @Param isPlayoff query bool false "Whether the stats are for playoffs"
// @Success 200 {object} controllers.PlayerTotalsResponse
// @Failure 500 {object} map[string]string
// @Router /api/playertotals [get]
func GetPlayerTotalStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var stats []models.PlayerTotalStat

		// --- FILTERS ---
		playerId := c.Query("playerId")
		if playerId == "" {
			playerId = c.Query("player_id")
		}

		season := c.QueryInt("season", 0)
		team := c.Query("team")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)

		// --- SORTING ---
		sortByParam := c.Query("sortBy", "points")
		ascending := c.QueryBool("ascending", false)

		// Translate sortBy param to a valid DB column, defaulting if not found.
		sortBy, ok := totalSortMap[sortByParam]
		if !ok {
			sortBy = "points" // Safe default
		}

		offset := (page - 1) * pageSize
		order := sortBy + " DESC"
		if ascending {
			order = sortBy + " ASC"
		}

		query := db.Model(&models.PlayerTotalStat{})

		if season != 0 {
			query = query.Where("season = ?", season)
		}
		if team != "" {
			query = query.Where("team = ?", team)
		}
		if playerId != "" {
			query = query.Where("player_id = ?", playerId)
		}

		isPlayoffStr := c.Query("isPlayoff")
		if isPlayoffStr != "" {
			isPlayoff := c.QueryBool("isPlayoff", false)
			query = query.Where("is_playoff = ?", isPlayoff)
		}

		var total int64
		query.Count(&total)

		err := query.Order(order).Limit(pageSize).Offset(offset).Find(&stats).Error
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// --- MODIFICATION START ---
		// Transform the database models into DTOs.
		playerTotalDTOs := make([]PlayerTotalStatDTO, len(stats))
		for i, s := range stats {
			playerTotalDTOs[i] = PlayerTotalStatDTO{
				PlayerID:        s.PlayerID,
				PlayerName:      s.PlayerName,
				Position:        s.Position,
				Age:             s.Age,
				Games:           s.Games,
				GamesStarted:    s.GamesStarted,
				MinutesPG:       s.MinutesPG,
				FieldGoals:      s.FieldGoals,
				FieldAttempts:   s.FieldAttempts,
				FieldPercent:    s.FieldPercent,
				ThreeFG:         s.ThreeFG,
				ThreeAttempts:   s.ThreeAttempts,
				ThreePercent:    s.ThreePercent,
				TwoFG:           s.TwoFG,
				TwoAttempts:     s.TwoAttempts,
				TwoPercent:      s.TwoPercent,
				EffectFGPercent: s.EffectFGPercent,
				FT:              s.FT,
				FTAttempts:      s.FTAttempts,
				FTPercent:       s.FTPercent,
				OffensiveRB:     s.OffensiveRB,
				DefensiveRB:     s.DefensiveRB,
				TotalRB:         s.TotalRB,
				Assists:         s.Assists,
				Steals:          s.Steals,
				Blocks:          s.Blocks,
				Turnovers:       s.Turnovers,
				PersonalFouls:   s.PersonalFouls,
				Points:          s.Points,
				Team:            s.Team,
				Season:          s.Season,
				IsPlayoff:       s.IsPlayoff,
			}
		}

		// Build the final response using the new DTOs and response struct.
		resp := PlayerTotalsResponse{
			Data: playerTotalDTOs,
		}
		resp.Pagination.Total = total
		resp.Pagination.Page = page
		resp.Pagination.PageSize = pageSize
		resp.Pagination.Pages = (total + int64(pageSize) - 1) / int64(pageSize)

		return c.JSON(resp)
		// --- MODIFICATION END ---
	}
}
