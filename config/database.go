package config

import (
    "log"
    "gorm.io/driver/sqlite" // Or Postgres
    "gorm.io/gorm"
    "github.com/nprasad2077/NBA_Go/models"
)

func InitDB() *gorm.DB {
    db, err := gorm.Open(sqlite.Open("/app/data/nba.db"), &gorm.Config{})
    if err != nil {
        log.Fatal("Failed to connect database")
    }

    db.AutoMigrate(&models.PlayerAdvancedStat{})
    db.AutoMigrate(&models.PlayerTotalStat{})

    return db
}