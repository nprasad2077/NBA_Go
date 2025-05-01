package main

import (
    "log"
    "github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/config"
	"github.com/nprasad2077/NBA_Go/routes" 
    "github.com/nprasad2077/NBA_Go/services"
    _"github.com/nprasad2077/NBA_Go/docs" // swag docs
	fiberswagger "github.com/swaggo/fiber-swagger"
)

func main() {
    app := fiber.New()
    db := config.InitDB()

    // Automatically fetch player stats on startup
    go func() {
        if err := services.FetchAndStorePlayerStats(db, 2025); err != nil {
            log.Println("Initial fetch failed:", err)
        } else {
            log.Println("Initial player stats fetch successful.")
        }
    }()



    routes.RegisterPlayerRoutes(app, db)

    app.Get("/swagger/*", fiberswagger.WrapHandler)

    app.Listen(":3001")
}