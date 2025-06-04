package config

import (
    "log"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "github.com/nprasad2077/NBA_Go/models"
    "github.com/nprasad2077/NBA_Go/utils/metrics"
)

func InitDB(shouldMigrate bool) *gorm.DB {
    db, err := gorm.Open(sqlite.Open("/app/data/nba.db"), &gorm.Config{})
    if err != nil {
        log.Fatalf("failed to connect database: %v", err)
    }
    metrics.DBOperationsTotal.WithLabelValues("connect", "database").Inc()

    if shouldMigrate {
        if err := db.AutoMigrate(&models.PlayerAdvancedStat{}); err != nil {
            log.Fatalf("migrate PlayerAdvancedStat: %v", err)
        }
        if err := db.AutoMigrate(&models.PlayerTotalStat{}); err != nil {
            log.Fatalf("migrate PlayerTotalStat: %v", err)
        }
        if err := db.AutoMigrate(&models.PlayerShotChart{}); err != nil {
            log.Fatalf("migrate PlayerShotChart: %v", err)
        }
        if err := db.AutoMigrate(&models.APIKey{}); err != nil {
            log.Fatalf("migrate APIKey: %v", err)
        }
        metrics.DBOperationsTotal.WithLabelValues("migrate", "database").Inc()
    }

    return db
}