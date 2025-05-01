package main

import (
    "log"
    "os"
    "time"
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/gofiber/fiber/v2"
    "github.com/nprasad2077/NBA_Go/config"
    "github.com/nprasad2077/NBA_Go/routes" 
    "github.com/nprasad2077/NBA_Go/services"
    _"github.com/nprasad2077/NBA_Go/docs" // swag docs
    fiberswagger "github.com/swaggo/fiber-swagger"
    "gorm.io/gorm"
)

// importData handles fetching and storing all player stats
func importData(db *gorm.DB) {
    log.Println("Starting data import process...")
    
    // Import advanced stats
    for season := 2020; season <= 2025; season++ {
        if err := services.FetchAndStorePlayerAdvancedStats(db, season); err != nil {
            log.Printf("Fetch failed for player advanced season %d: %v\n", season, err)
        } else {
            log.Printf("Fetch successful for player advanced season %d\n", season)
        }
        time.Sleep(1100 * time.Millisecond) // optional delay
    }
    log.Printf("Player advanced import completed")
    
    // Import total stats
    for season := 2020; season <= 2025; season++ {
        if err := services.FetchAndStorePlayerTotalStats(db, season); err != nil {
            log.Printf("Fetch failed for player totals season %d: %v\n", season, err)
        } else {
            log.Printf("Fetch successful for player totals season %d\n", season)
        }
        time.Sleep(1000 * time.Millisecond) // optional delay
    }
    log.Printf("Player totals import completed")
    
    log.Println("Data import process completed successfully!")
}

func main() {
    // Check if we're running in import-data mode
    if len(os.Args) > 1 && os.Args[1] == "import-data" {
        db := config.InitDB()
        importData(db)
        return // Exit after import is complete
    }
    
    // Regular API server mode
    app := fiber.New()
    app.Use(logger.New())
    
    db := config.InitDB()
    
    // No automatic data fetching in API server mode
    // Only the db-init container will handle data imports

    routes.RegisterPlayerAdvancedRoutes(app, db)
    routes.RegisterPlayerTotalRoutes(app, db)

    app.Get("/swagger/*", fiberswagger.WrapHandler)
    
    log.Println("Starting API server on port 5000...")
    app.Listen(":5000")
}