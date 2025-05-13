package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/controllers"
	"gorm.io/gorm"
)

func RegisterPlayerTotalRoutes(app *fiber.App, db *gorm.DB) {
	api := app.Group("/api/playertotals")

	api.Get("/fetch", controllers.FetchPlayerTotalStats(db))
	api.Get("/scrape", controllers.ScrapePlayerTotalStats(db))
	api.Get("/", controllers.GetPlayerTotalStats(db))
}