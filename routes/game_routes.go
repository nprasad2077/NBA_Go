package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/controllers"
	"github.com/nprasad2077/NBA_Go/utils/middleware"
	"gorm.io/gorm"
)

func RegisterGameRoutes(app *fiber.App, db *gorm.DB) {
	// Create a new group for game-related endpoints
	api := app.Group("/api/games")

	// Register the GET endpoint to fetch games with semi-dynamic edge caching (1-day edge TTL)
	api.Get("/", middleware.EdgeCache(middleware.SemiDynamic, "nba-games"), controllers.GetGames(db))
}