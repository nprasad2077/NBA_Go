// File: config/database.go

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
	// ... (DSN setup code is unchanged) ...
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	metrics.DBOperationsTotal.WithLabelValues("connect", "database").Inc()

	if shouldMigrate {
		// --- ADD THIS BLOCK TO DROP STALE TABLES ---
		// Drop tables in reverse order of dependency (children first).
		// This ensures a clean migration every time the import process runs.
		log.Println("⚠️ Dropping existing game-related tables for a clean migration...")
		if err := db.Migrator().DropTable(
			&models.LineScore{},
			&models.PlayerGameBasicStat{},
			&models.PlayerGameAdvStat{},
			&models.TeamGameBasicStat{},
			&models.TeamGameAdvStat{},
			&models.Game{}, // Drop parent table last
		); err != nil {
			log.Fatalf("failed to drop tables: %v", err)
		}
		log.Println("✅ Tables dropped successfully.")
		// --- END OF ADDED BLOCK ---


		// Your existing AutoMigrate calls will now work correctly
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
		// Game Models will be recreated with the correct schema
		if err := db.AutoMigrate(
			&models.Game{},
			&models.LineScore{},
			&models.PlayerGameBasicStat{},
			&models.PlayerGameAdvStat{},
			&models.TeamGameBasicStat{},
			&models.TeamGameAdvStat{},
		); err != nil {
			log.Fatalf("migrate game models: %v", err)
		}
		metrics.DBOperationsTotal.WithLabelValues("migrate", "database").Inc()
	}

	return db
}