package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/controllers"
	"github.com/nprasad2077/NBA_Go/utils/middleware"
	"gorm.io/gorm"
)

func RegisterPlayerTotalRoutes(app *fiber.App, db *gorm.DB) {
	api := app.Group("/api/playertotals")

	// Scraper trigger - never cached
	api.Get("/scrape", middleware.EdgeCache(middleware.NoCache), controllers.ScrapePlayerTotalStats(db))
	// Public query endpoint - edge cached with stale-while-revalidate
	api.Get("/", middleware.EdgeCache(middleware.SemiDynamic, "nba-player-totals"), controllers.GetPlayerTotalStats(db))
}