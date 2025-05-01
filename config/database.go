package config

import (
    "log"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "github.com/nprasad2077/NBA_Go/models"
    "github.com/nprasad2077/NBA_Go/utils/metrics"
)

func InitDB() *gorm.DB {
    db, err := gorm.Open(sqlite.Open("/app/data/nba.db"), &gorm.Config{})
    if err != nil {
        log.Printf("/app/config/database.go:11\n[error] failed to initialize database, got error %v", err)
        log.Fatal("Failed to connect database")
    }
    
    // Track database operations
    metrics.DBOperationsTotal.WithLabelValues("connect", "database").Inc()
    
    // Auto migrate models
    db.AutoMigrate(&models.PlayerAdvancedStat{})
    db.AutoMigrate(&models.PlayerTotalStat{})
    
    metrics.DBOperationsTotal.WithLabelValues("migrate", "database").Inc()
    
    return db
}