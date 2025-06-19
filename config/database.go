package config

import (
	"fmt"
	"log"
	"os"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils/metrics"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDB(shouldMigrate bool) *gorm.DB {
	// CHANGE: Build the DSN from environment variables
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) // <- CHANGE: Use the postgres driver
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