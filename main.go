package main

import (
    "log"
    "time"
    
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/gofiber/fiber/v2/middleware/adaptor"
    "github.com/nprasad2077/NBA_Go/config"
    "github.com/nprasad2077/NBA_Go/routes"
    "github.com/nprasad2077/NBA_Go/services"
    "github.com/nprasad2077/NBA_Go/utils/middleware"
    _ "github.com/nprasad2077/NBA_Go/docs"
    fiberswagger "github.com/swaggo/fiber-swagger"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    app := fiber.New()

    // Add standard logger middleware
    app.Use(logger.New())
    
    // Add metrics middleware
    app.Use(middleware.MetricsMiddleware())

    // Initialize database
    db := config.InitDB()

    // Original data fetching code, no environment variable check
    go func() {
        for season := 1993; season <= 2025; season++ {
            if err := services.FetchAndStorePlayerAdvancedStats(db, season); err != nil {
                log.Printf("Fetch failed for player advanced season %d: %v\n", season, err)
            } else {
                log.Printf("Fetch successful for player advanced season %d\n", season)
            }
            time.Sleep(1100 * time.Millisecond) // optional delay
        }
        log.Printf("player advanced Import Success")
    }()

    go func() {
        for season := 1993; season <= 2025; season++ {
            if err := services.FetchAndStorePlayerTotalStats(db, season); err != nil {
                log.Printf("Fetch failed for player totals season %d: %v\n", season, err)
            } else {
                log.Printf("Fetch successful for player totals season %d\n", season)
            }
            time.Sleep(1000 * time.Millisecond) // optional delay
        }
        log.Printf("player totals Import Success")
    }()

    // Add Prometheus metrics endpoint
    app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

    // Register routes
    routes.RegisterPlayerAdvancedRoutes(app, db)
    routes.RegisterPlayerTotalRoutes(app, db)

    // Swagger endpoint
    app.Get("/swagger/*", fiberswagger.WrapHandler)

    app.Listen(":5000")
}