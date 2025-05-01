package routes

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"github.com/nprasad2077/NBA_Go/controllers"
)

func RegisterPlayerAdvancedRoutes(app *fiber.App, db *gorm.DB) {
	api := app.Group("/api/playeradvancedstats")

	api.Get("/fetch", controllers.FetchPlayerAdvancedStats(db))
	api.Get("/", controllers.GetAllAdvancedPlayerStats(db)) // ✅ Add this line
}