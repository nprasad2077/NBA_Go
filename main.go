package main

import (
    "log"
    "time"
    "github.com/gofiber/fiber/v2/middleware/logger"
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
        for season := 2020; season <= 2025; season++ {
            if err := services.FetchAndStorePlayerAdvancedStats(db, season); err != nil {
                log.Printf("Fetch failed for player advanced  season %d: %v\n", season, err)
            } else {
                log.Printf("Fetch successful for player advanced season %d\n", season)
            }
            time.Sleep(2 * time.Second) // optional delay
        }
        log.Printf("player advanced Import Success")
    }()

    go func() {
        for season := 2020; season <= 2025; season++ {
            if err := services.FetchAndStorePlayerTotalStats(db, season); err != nil {
                log.Printf("Fetch failed for player totals season %d: %v\n", season, err)
            } else {
                log.Printf("Fetch successful for player totals  season %d\n", season)
            }
            time.Sleep(1 * time.Second) // optional delay
        }
        log.Printf("player totals Import Success")
    }()

    



    routes.RegisterPlayerAdvancedRoutes(app, db)
    routes.RegisterPlayerTotalRoutes(app, db)

    app.Get("/swagger/*", fiberswagger.WrapHandler)

    app.Use(logger.New())

    app.Listen(":3001")
}