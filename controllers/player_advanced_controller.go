package controllers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/services"
)

// --- MODIFICATION START ---
// A DTO is created to control the JSON output for advanced stats.
// It omits fields like ID, ExternalID, CreatedAt, UpdatedAt, and DeletedAt.
type PlayerAdvancedStatDTO struct {
	PlayerID           string  `json:"playerId"`
	PlayerName         string  `json:"playerName"`
	Position           string  `json:"position"`
	Age                int     `json:"age"`
	Team               string  `json:"team"`
	Games              int     `json:"games"`
	MinutesPlayed      int     `json:"minutesPlayed"`
	PER                float64 `json:"per"`
	TSPercent          float64 `json:"tsPercent"`
	ThreePAR           float64 `json:"threePAR"`
	FTr                float64 `json:"ftr"`
	OffensiveRBPercent float64 `json:"offensiveRBPercent"`
	DefensiveRBPercent float64 `json:"defensiveRBPercent"`
	TotalRBPercent     float64 `json:"totalRBPercent"`
	AssistPercent      float64 `json:"assistPercent"`
	StealPercent       float64 `json:"stealPercent"`
	BlockPercent       float64 `json:"blockPercent"`
	TurnoverPercent    float64 `json:"turnoverPercent"`
	UsagePercent       float64 `json:"usagePercent"`
	OffensiveWS        float64 `json:"offensiveWS"`
	DefensiveWS        float64 `json:"defensiveWS"`
	WinShares          float64 `json:"winShares"`
	WinSharesPer       float64 `json:"winSharesPer"`
	OffensiveBox       float64 `json:"offensiveBox"`
	DefensiveBox       float64 `json:"defensiveBox"`
	Box                float64 `json:"box"`
	VORP               float64 `json:"vorp"`
	Season             int     `json:"season"`
	IsPlayoff          bool    `json:"isPlayoff"`
}

// AdvancedStatsResponse is the swagger response model for GetAllAdvancedPlayerStats
// It wraps the returned player advanced stats and pagination metadata.
// The Data field is updated to use the DTO.
type AdvancedStatsResponse struct {
	Data       []PlayerAdvancedStatDTO `json:"data"`
	Pagination struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Pages    int64 `json:"pages"`
	} `json:"pagination"`
}
// --- MODIFICATION END ---


var advancedSortMap = map[string]string{
	"playerId":           "player_id",
	"playerName":         "player_name",
	"position":           "position",
	"age":                "age",
	"games":              "games",
	"minutesPlayed":      "minutes_played",
	"per":                "per",
	"tsPercent":          "ts_percent",
	"threePAR":           "three_par",
	"ftr":                "ftr",
	"offensiveRBPercent": "offensive_rb_percent",
	"defensiveRBPercent": "defensive_rb_percent",
	"totalRBPercent":     "total_rb_percent",
	"assistPercent":      "assist_percent",
	"stealPercent":       "steal_percent",
	"blockPercent":       "block_percent",
	"turnoverPercent":    "turnover_percent",
	"usagePercent":       "usage_percent",
	"offensiveWS":        "offensive_ws",
	"defensiveWS":        "defensive_ws",
	"winShares":          "win_shares",
	"winSharesPer":       "win_shares_per",
	"offensiveBox":       "offensive_box",
	"defensiveBox":       "defensive_box",
	"box":                "box",
	"vorp":               "vorp",
	"team":               "team",
	"season":             "season",
}


// ScrapePlayerAdvancedStats godoc
// @ignore
// @Summary     Scrape player advanced stats from BR website
// @Tags        PlayerStats
// @Param       season    query  int  true  "Season (e.g. 2025)"
// @Param       isPlayoff query  bool false "Whether playoffs?"
// @Success     200       {object} map[string]string
// @Failure     400,500   {object} map[string]string
// //@Router      /api/playeradvancedstats/scrape [get]
func ScrapePlayerAdvancedStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		season := c.QueryInt("season", 0)
		if season == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "season is required"})
		}
		isPlayoff := c.QueryBool("isPlayoff", false)

		if err := services.FetchAndStorePlayerAdvancedScrapedStats(db, season, isPlayoff); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "scrape+store complete"})
	}
}

// GetAllAdvancedPlayerStats godoc
// //@Security    ApiKeyAuth
// @Summary     Get player advanced stats
// @Description Returns filtered and paginated player advanced stats
// @Tags        PlayerStats
// @x-order 2
// @Group Player Stats
// @Accept      json
// @Produce     json
// @Param       season     query  int     false  "Season (e.g., 2025)"
// @Param       team       query  string  false  "Team abbreviation (e.g., MIL)"
// @Param       playerId   query  string  false  "Player ID (e.g., greenaj01)"
// @Param       page       query  int     false  "Page number"       default(1)
// @Param       pageSize   query  int     false  "Page size"         default(20)
// @Param       sortBy     query  string  false  "Field to sort by"  default(winShares)
// @Param       ascending  query  bool    false  "Sort ascending"    default(false)
// @Param       isPlayoff  query  bool    false  "Whether playoffs?"
// @Success     200        {object} controllers.AdvancedStatsResponse
// @Failure     500        {object} map[string]string
// @Router      /api/playeradvancedstats [get]
func GetAllAdvancedPlayerStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var stats []models.PlayerAdvancedStat

		// --- FILTERS ---
		playerId := c.Query("playerId")
		if playerId == "" {
			playerId = c.Query("player_id")
		}

		season := c.QueryInt("season", 0)
		team := c.Query("team")

		// --- PAGINATION ---
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)
		offset := (page - 1) * pageSize

		// --- SORTING ---
		sortByParam := c.Query("sortBy", "winShares")
		ascending := c.QueryBool("ascending", false)

		sortBy, ok := advancedSortMap[sortByParam]
		if !ok {
			sortBy = "win_shares" // Safe default
		}

		order := sortBy + " DESC"
		if ascending {
			order = sortBy + " ASC"
		}

		// --- QUERY BUILDING ---
		query := db.Model(&models.PlayerAdvancedStat{})

		if season != 0 {
			query = query.Where("season = ?", season)
		}
		if team != "" {
			query = query.Where("team = ?", team)
		}
		if playerId != "" {
			query = query.Where("player_id = ?", playerId)
		}

		if c.Query("isPlayoff") != "" {
			isPlayoff := c.QueryBool("isPlayoff", false)
			query = query.Where("is_playoff = ?", isPlayoff)
		}

		// Count total records for pagination
		var total int64
		query.Count(&total)

		// Fetch the data page
		err := query.Order(order).Limit(pageSize).Offset(offset).Find(&stats).Error
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// --- MODIFICATION START ---
		// Transform the database models into DTOs.
		advancedStatDTOs := make([]PlayerAdvancedStatDTO, len(stats))
		for i, s := range stats {
			advancedStatDTOs[i] = PlayerAdvancedStatDTO{
				PlayerID:           s.PlayerID,
				PlayerName:         s.PlayerName,
				Position:           s.Position,
				Age:                s.Age,
				Team:               s.Team,
				Games:              s.Games,
				MinutesPlayed:      s.MinutesPlayed,
				PER:                s.PER,
				TSPercent:          s.TSPercent,
				ThreePAR:           s.ThreePAR,
				FTr:                s.FTR, // FIX: Changed s.FTr to s.FTR to match the model
				OffensiveRBPercent: s.OffensiveRBPercent,
				DefensiveRBPercent: s.DefensiveRBPercent,
				TotalRBPercent:     s.TotalRBPercent,
				AssistPercent:      s.AssistPercent,
				StealPercent:       s.StealPercent,
				BlockPercent:       s.BlockPercent,
				TurnoverPercent:    s.TurnoverPercent,
				UsagePercent:       s.UsagePercent,
				OffensiveWS:        s.OffensiveWS,
				DefensiveWS:        s.DefensiveWS,
				WinShares:          s.WinShares,
				WinSharesPer:       s.WinSharesPer,
				OffensiveBox:       s.OffensiveBox,
				DefensiveBox:       s.DefensiveBox,
				Box:                s.Box,
				VORP:               s.VORP,
				Season:             s.Season,
				IsPlayoff:          s.IsPlayoff,
			}
		}

		// Build the final response using the new DTOs and response struct.
		resp := AdvancedStatsResponse{
			Data: advancedStatDTOs,
		}
		resp.Pagination.Total = total
		resp.Pagination.Page = page
		resp.Pagination.PageSize = pageSize
		resp.Pagination.Pages = (total + int64(pageSize) - 1) / int64(pageSize)

		return c.JSON(resp)
		// --- MODIFICATION END ---
	}
}
