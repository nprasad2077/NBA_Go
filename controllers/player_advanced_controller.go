package controllers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/services"
)

var advancedSortMap = map[string]string{
    "winShares":   "win_shares",
    "per":         "per",
    "tsPercent":   "ts_percent",
    "playerId":    "player_id",
    "season":      "season",
    "team":        "team",
}

// AdvancedStatsResponse is the swagger response model for GetAllAdvancedPlayerStats
// It wraps the returned player advanced stats and pagination metadata.
type AdvancedStatsResponse struct {
	Data       []models.PlayerAdvancedStat `json:"data"`
	Pagination struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Pages    int64 `json:"pages"`
	} `json:"pagination"`
}

// FetchPlayerAdvancedStats returns a handler that imports advanced stats for a season
// Note: not exposed in Swagger docs, only internal use.
// func FetchPlayerAdvancedStats(db *gorm.DB) fiber.Handler {
// 	return func(c *fiber.Ctx) error {
// 		season := c.QueryInt("season", 2025)
// 		isPlayoff := c.QueryBool("isPlayoff", false)

// 		if err := services.FetchAndStorePlayerAdvancedStats(db, season, isPlayoff); err != nil {
// 			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
// 		}

// 		return c.JSON(fiber.Map{"message": "Player stats fetched and saved."})
// 	}
// }

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

		// --- MODIFICATION FOR FILTERS ---
        // Allow both "playerId" and "player_id"
        playerId := c.Query("playerId")
        if playerId == "" {
            playerId = c.Query("player_id")
        }

		// Filters
		season := c.QueryInt("season", 0)
		team := c.Query("team")
		// playerId := c.Query("playerId")

		// Pagination
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)
		offset := (page - 1) * pageSize

		// --- MODIFICATION FOR SORTING ---
        // Sorting
        sortByParam := c.Query("sortBy", "winShares") // Default to a common field
        ascending := c.QueryBool("ascending", false)

        // Translate sortBy param to a valid DB column, defaulting if not found.
        sortBy, ok := advancedSortMap[sortByParam]
        if !ok {
            sortBy = "win_shares" // Safe default
        }

        order := sortBy + " DESC"
        if ascending {
            order = sortBy + " ASC"
        }

		// Build query
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

		// Count total
		var total int64
		query.Count(&total)

		// Fetch page
		err := query.Order(order).Limit(pageSize).Offset(offset).Find(&stats).Error
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Build response
		resp := AdvancedStatsResponse{Data: stats}
		resp.Pagination.Total = total
		resp.Pagination.Page = page
		resp.Pagination.PageSize = pageSize
		resp.Pagination.Pages = (total + int64(pageSize) - 1) / int64(pageSize)

		return c.JSON(resp)
	}
}
