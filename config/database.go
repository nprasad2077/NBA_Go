// File: config/database.go

package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/utils/metrics"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

type DBConfig struct {
	User      string
	Password  string
	DBName    string
	SSLMode   string
	WriteHost string
	WritePort string
	ReadHost  string
	ReadPort  string
}

func LoadDBConfig() DBConfig {
	user := getEnvOrDefault("DB_USER", "postgres")
	pass := getEnvOrDefault("DB_PASSWORD", "postgrespassword")
	dbname := getEnvOrDefault("DB_NAME", "appdb")
	ssl := getEnvOrDefault("DB_SSLMODE", "disable")

	// Base fallback
	baseHost := getEnvOrDefault("DB_HOST", "178.105.149.129")
	basePort := os.Getenv("DB_PORT")

	writeHost := getEnvOrDefault("DB_WRITE_HOST", baseHost)
	writePort := getEnvOrDefault("DB_WRITE_PORT", "5437")
	if os.Getenv("DB_WRITE_PORT") == "" && basePort != "" {
		writePort = basePort
	}

	readHost := getEnvOrDefault("DB_READ_HOST", baseHost)
	readPort := getEnvOrDefault("DB_READ_PORT", "5438")
	if os.Getenv("DB_READ_PORT") == "" && basePort != "" {
		readPort = basePort
	}

	return DBConfig{
		User:      user,
		Password:  pass,
		DBName:    dbname,
		SSLMode:   ssl,
		WriteHost: writeHost,
		WritePort: writePort,
		ReadHost:  readHost,
		ReadPort:  readPort,
	}
}

func buildDSN(host, port, user, pass, dbname, ssl string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		host, port, user, pass, dbname, ssl)
}

func InitDB(shouldMigrate bool) *gorm.DB {
	cfg := LoadDBConfig()
	writeDSN := buildDSN(cfg.WriteHost, cfg.WritePort, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
	readDSN := buildDSN(cfg.ReadHost, cfg.ReadPort, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	slog.Info("🔌 Connecting to PostgreSQL Cluster via HAProxy",
		"write_ingress", fmt.Sprintf("%s:%s", cfg.WriteHost, cfg.WritePort),
		"read_ingress", fmt.Sprintf("%s:%s", cfg.ReadHost, cfg.ReadPort),
		"database", cfg.DBName,
	)

	// Primary GORM instance connects to Write Ingress (:5437)
	db, err := gorm.Open(postgres.Open(writeDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		slog.Error("❌ Failed to connect to Primary/Write database ingress", "error", err)
		os.Exit(1)
	}

	// Register DBResolver for automatic Read/Write splitting
	resolverCfg := dbresolver.Config{
		Sources:  []gorm.Dialector{postgres.Open(writeDSN)},
		Replicas: []gorm.Dialector{postgres.Open(readDSN)},
		Policy:   dbresolver.RandomPolicy{}, // HAProxy handles round-robin balancing across replicas
	}

	err = db.Use(dbresolver.Register(resolverCfg).
		SetConnMaxIdleTime(30 * time.Minute).
		SetConnMaxLifetime(2 * time.Hour).
		SetMaxIdleConns(10).
		SetMaxOpenConns(50),
	)
	if err != nil {
		slog.Error("❌ Failed to configure DBResolver plugin", "error", err)
		os.Exit(1)
	}

	metrics.DBOperationsTotal.WithLabelValues("connect", "database").Inc()

	// Start asynchronous connection pool metrics collection
	metrics.StartDBPoolMetricsCollector(db)

	if shouldMigrate {
		slog.Info("🚀 Executing database AutoMigrate on Primary (:5437)...")
		modelsToMigrate := []interface{}{
			&models.APIKey{},
			&models.PlayerAdvancedStat{},
			&models.PlayerTotalStat{},
			&models.PlayerShotChart{},
			&models.Game{},
			&models.LineScore{},
			&models.PlayerGameBasicStat{},
			&models.PlayerGameAdvStat{},
			&models.TeamGameBasicStat{},
			&models.TeamGameAdvStat{},
		}
		for _, m := range modelsToMigrate {
			if err := db.Clauses(dbresolver.Write).AutoMigrate(m); err != nil {
				slog.Error("❌ Migration failed", "model", fmt.Sprintf("%T", m), "error", err)
				os.Exit(1)
			}
		}
		metrics.DBOperationsTotal.WithLabelValues("migrate", "database").Inc()
		slog.Info("✅ Database schema migration completed successfully")
	}

	return db
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}