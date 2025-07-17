package routes

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"github.com/nprasad2077/NBA_Go/controllers"
)

func RegisterGameRoutes(app *fiber.App, db *gorm.DB) {
	// Create a new group for game-related endpoints
	api := app.Group("/api/games")

	// Register the GET endpoint to fetch games
	api.Get("/", controllers.GetGames(db))
}