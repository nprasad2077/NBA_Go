// FetchPlayerTotalStats godoc
// @Summary Fetch player total stats from external API
// @Description Imports totals data and stores or updates in DB
// @Tags PlayerTotals
// @Accept  json
// @Produce  json
// @Param season query int false "Season (e.g. 2000)"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/playertotals/fetch [get]

package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/services"
	"gorm.io/gorm"
	"github.com/nprasad2077/NBA_Go/models"
)

func FetchPlayerTotalStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		season := c.QueryInt("season", 2025)

		err := services.FetchAndStorePlayerTotalStats(db, season)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"message": "Player total stats fetched and saved."})
	}
}


// GetPlayerTotalStats godoc
// @Security ApiKeyAuth 
// @Summary Get player total stats
// @Description Filter and paginate player totals
// @Tags PlayerTotals
// @Accept  json
// @Produce  json
// @Param season query int false "Season (e.g. 2000)"
// @Param team query string false "Team abbreviation (e.g. LAL)"
// @Param playerId query string false "Player ID (e.g. greenac01)"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Param sortBy query string false "Field to sort by (e.g. points, assists)"
// @Param ascending query bool false "Sort ascending (default false)"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /api/playertotals [get]
func GetPlayerTotalStats(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var stats []models.PlayerTotalStat

		season := c.QueryInt("season", 0)
		team := c.Query("team")
		playerId := c.Query("playerId")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("pageSize", 20)
		sortBy := c.Query("sortBy", "points")
		ascending := c.QueryBool("ascending", false)
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

		var total int64
		query.Count(&total)

		err := query.Order(order).Limit(pageSize).Offset(offset).Find(&stats).Error
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

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