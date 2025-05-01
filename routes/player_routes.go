package routes

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"github.com/nprasad2077/NBA_Go/controllers"
)

func RegisterPlayerRoutes(app *fiber.App, db *gorm.DB) {
	api := app.Group("/api/playerstats")

	api.Get("/fetch", controllers.FetchPlayerStats(db))
	api.Get("/", controllers.GetAllPlayerStats(db)) // ✅ Add this line
}