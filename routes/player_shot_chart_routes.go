package routes

import (
    "github.com/gofiber/fiber/v2"
    "github.com/nprasad2077/NBA_Go/controllers"
    "gorm.io/gorm"
)

func RegisterPlayerShotChartRoutes(app *fiber.App, db *gorm.DB) {
    api := app.Group("/api/playershotchart")
    api.Get("/fetch", controllers.FetchPlayerShotChart(db))
    api.Get("/", controllers.GetPlayerShotChart(db))
}