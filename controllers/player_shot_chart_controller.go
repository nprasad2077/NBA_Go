// NBA_Go/controllers/player_shot_chart_controller.go
package controllers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/nprasad2077/NBA_Go/models"
    "github.com/nprasad2077/NBA_Go/services"
    "gorm.io/gorm"
)

// FetchPlayerShotChart godoc
// @Summary     Fetch a single player's shot-chart data from external API
// @Description Imports shot-chart data for the given playerId and stores/updates in DB
// @Tags        PlayerShotChart
// @Accept      json
// @Produce     json
// @Param       playerId query  string true "Player ID (e.g., hardeja01)"
// @Success     200      {object} map[string]string
// @Failure     400      {object} map[string]string
// @Failure     500      {object} map[string]string
// @Router      /api/playershotchart/fetch [get]
func FetchPlayerShotChart(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        pid := c.Query("playerId")
        if pid == "" {
            return c.Status(400).JSON(fiber.Map{
                "error": "playerId query parameter is required",
            })
        }
        if err := services.FetchAndStoreShotChartForPlayer(db, pid); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(fiber.Map{"message": "Shot chart for " + pid + " fetched and saved."})
    }
}

// GetPlayerShotChart godoc
// @Security    ApiKeyAuth
// @Summary     Get shot-chart data
// @Description Returns shot-chart points, optionally filtered by playerId and/or season
// @Tags        PlayerShotChart
// @Accept      json
// @Produce     json
// @Param       playerId query  string false "Player ID (e.g., hardeja01)"
// @Param       season   query  int    false "Season (e.g., 2023)"
// @Success     200      {array}  models.PlayerShotChart
// @Failure     500      {object} map[string]string
// @Router      /api/playershotchart [get]
func GetPlayerShotChart(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        var shots []models.PlayerShotChart

        // Start with the base model
        query := db.Model(&models.PlayerShotChart{})

        // Optional filter: playerId
        if pid := c.Query("playerId"); pid != "" {
            query = query.Where("player_id = ?", pid)
        }

        // Optional filter: season
        if s := c.QueryInt("season", 0); s != 0 {
            query = query.Where("season = ?", s)
        }

        // Execute
        if err := query.Find(&shots).Error; err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        return c.JSON(shots)
    }
}