package routes

import (
    "github.com/gofiber/fiber/v2"
    "github.com/nprasad2077/NBA_Go/controllers"
    "github.com/nprasad2077/NBA_Go/utils/middleware"
    "gorm.io/gorm"
)

// RegisterPlayerShotChartRoutes sets up the shot-chart endpoints
func RegisterPlayerShotChartRoutes(app *fiber.App, db *gorm.DB) {
    api := app.Group("/api/playershotchart")
    // Scraper trigger - never cached
    api.Get("/scrape", middleware.EdgeCache(middleware.NoCache), controllers.ScrapePlayerShotChart(db))
    // Public shot chart query endpoint - immutable 30-day edge cache
    api.Get("/", middleware.EdgeCache(middleware.Immutable, "nba-shot-charts"), controllers.GetPlayerShotChart(db))
}