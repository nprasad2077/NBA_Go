package controllers

import (
    "github.com/gofiber/fiber/v2"
    "gorm.io/gorm"
    "github.com/nprasad2077/NBA_Go/services"
    "github.com/nprasad2077/NBA_Go/models"
)

func FetchPlayerAdvancedStats(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        season := c.QueryInt("season", 2025)

        // Call service and assign err here
        err := services.FetchAndStorePlayerAdvancedStats(db, season)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(fiber.Map{"message": "Player stats fetched and saved."})
    }
}



// GetAllPlayerStats godoc
// @Security ApiKeyAuth 
// @Summary Get player stats
// @Description Returns filtered and paginated player stats
// @Tags PlayerStats
// @Accept json
// @Produce json
// @Param season query int false "Season (e.g., 2025)"
// @Param team query string false "Team abbreviation (e.g., MIL)"
// @Param playerId query string false "Player ID (e.g., greenaj01)"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param sortBy query string false "Field to sort by (e.g., per, games, winShares)"
// @Param ascending query bool false "Sort ascending (default false)"
// @Success 200 {object} map[string]interface{}
// @Router /api/playeradvancedstats [get]
func GetAllAdvancedPlayerStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var stats []models.PlayerAdvancedStat

		// Filters
		season := c.QueryInt("season", 0)
		team := c.Query("team")
		playerId := c.Query("playerId")

		// Pagination
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)
		offset := (page - 1) * pageSize

		// Sorting
		sortBy := c.Query("sortBy", "win_shares") // default field
		ascending := c.QueryBool("ascending", false)
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

		var total int64
		query.Count(&total) // get total count before pagination

		err := query.Order(order).Limit(pageSize).Offset(offset).Find(&stats).Error
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Response with metadata
		return c.JSON(fiber.Map{
			"data": stats,
			"pagination": fiber.Map{
				"total":    total,
				"page":     page,
				"pageSize": pageSize,
				"pages":    (total + int64(pageSize) - 1) / int64(pageSize),
			},
		})
	}
}