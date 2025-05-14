package routes

import (
    "github.com/gofiber/fiber/v2"
    "github.com/nprasad2077/NBA_Go/controllers"
    "gorm.io/gorm"
)

// RegisterPlayerShotChartRoutes sets up the shot-chart endpoints
func RegisterPlayerShotChartRoutes(app *fiber.App, db *gorm.DB) {
    api := app.Group("/api/playershotchart")
    api.Get("/fetch",  controllers.FetchPlayerShotChartAPI(db))
    api.Get("/scrape", controllers.ScrapePlayerShotChart(db))
    api.Get("/",        controllers.GetPlayerShotChart(db))
}