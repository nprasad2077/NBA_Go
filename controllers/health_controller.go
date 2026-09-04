package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

type TargetHealth struct {
	Status    string  `json:"status"`
	Target    string  `json:"target"`
	LatencyMs float64 `json:"latencyMs"`
	Error     string  `json:"error,omitempty"`
}

type ReadinessResponse struct {
	Status    string                  `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Targets   map[string]TargetHealth `json:"targets"`
}

// RegisterHealthRoutes mounts /healthz and /readyz probes
func RegisterHealthRoutes(app *fiber.App, db *gorm.DB) {
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "UP",
			"timestamp": time.Now().UTC(),
		})
	})

	app.Get("/readyz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		targets := make(map[string]TargetHealth)
		overallHealthy := true

		writeHost := os.Getenv("DB_WRITE_HOST")
		if writeHost == "" {
			writeHost = "178.105.149.129"
		}
		writePort := os.Getenv("DB_WRITE_PORT")
		if writePort == "" {
			writePort = "5437"
		}

		readHost := os.Getenv("DB_READ_HOST")
		if readHost == "" {
			readHost = "178.105.149.129"
		}
		readPort := os.Getenv("DB_READ_PORT")
		if readPort == "" {
			readPort = "5438"
		}

		// Check Write Ingress (:5437)
		writeTarget := fmt.Sprintf("%s:%s", writeHost, writePort)
		writeStart := time.Now()
		writeDB, err := db.Clauses(dbresolver.Write).DB()
		if err != nil || writeDB == nil || writeDB.PingContext(ctx) != nil {
			overallHealthy = false
			errMsg := "Write ingress unreachable"
			if err != nil {
				errMsg = err.Error()
			}
			targets["db_write"] = TargetHealth{
				Status: "DOWN",
				Target: writeTarget,
				Error:  errMsg,
			}
		} else {
			targets["db_write"] = TargetHealth{
				Status:    "UP",
				Target:    writeTarget,
				LatencyMs: float64(time.Since(writeStart).Microseconds()) / 1000.0,
			}
		}

		// Check Read Ingress (:5438)
		readTarget := fmt.Sprintf("%s:%s", readHost, readPort)
		readStart := time.Now()
		readDB, err := db.Clauses(dbresolver.Read).DB()
		if err != nil || readDB == nil || readDB.PingContext(ctx) != nil {
			overallHealthy = false
			errMsg := "Read ingress unreachable"
			if err != nil {
				errMsg = err.Error()
			}
			targets["db_read"] = TargetHealth{
				Status: "DOWN",
				Target: readTarget,
				Error:  errMsg,
			}
		} else {
			targets["db_read"] = TargetHealth{
				Status:    "UP",
				Target:    readTarget,
				LatencyMs: float64(time.Since(readStart).Microseconds()) / 1000.0,
			}
		}

		statusCode := fiber.StatusOK
		statusText := "UP"
		if !overallHealthy {
			statusCode = fiber.StatusServiceUnavailable
			statusText = "DEGRADED"
		}

		return c.Status(statusCode).JSON(ReadinessResponse{
			Status:    statusText,
			Timestamp: time.Now().UTC(),
			Targets:   targets,
		})
	})
}
