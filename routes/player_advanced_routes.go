package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/controllers"
	"github.com/nprasad2077/NBA_Go/utils/middleware"
	"gorm.io/gorm"
)

func RegisterPlayerAdvancedRoutes(app *fiber.App, db *gorm.DB) {
	api := app.Group("/api/playeradvancedstats")

	// Scraper trigger - never cached
	api.Get("/scrape", middleware.EdgeCache(middleware.NoCache), controllers.ScrapePlayerAdvancedStats(db))
	// Public query endpoint - edge cached with stale-while-revalidate
	api.Get("/", middleware.EdgeCache(middleware.SemiDynamic, "nba-player-advanced"), controllers.GetAllAdvancedPlayerStats(db))
}