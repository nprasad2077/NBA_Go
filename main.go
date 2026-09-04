// @title       NBA_Go API
// @version     1.0
// @description Stats service, now with public access!
// @schemes     https http
// //@schemes     http
// @BasePath    /
//
// @tag.name    Games
// @tag.name    PlayerTotals
// @tag.name    PlayerStats
// @tag.name    PlayerShotChart
// To re-enable security docs:
// 1. Uncomment the securityDefinitions block below.
// 2. Uncomment the @Security annotation in each controller.
// 3. Run 'swag init'.
//
// //@securityDefinitions.apikey ApiKeyAuth
// //@in   header
// //@name X-API-Key
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	fiberswagger "github.com/swaggo/fiber-swagger"

	"github.com/nprasad2077/NBA_Go/config"
	"github.com/nprasad2077/NBA_Go/controllers"
	_ "github.com/nprasad2077/NBA_Go/docs"
	"github.com/nprasad2077/NBA_Go/routes"
	"github.com/nprasad2077/NBA_Go/utils/middleware"
)

func main() {
	// Setup structured JSON logger
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(jsonHandler))

	// ——— One-off import-data mode ———
	if len(os.Args) > 1 && os.Args[1] == "import-data" {
		// Run all DB migrations + import steps exactly once on Primary (Write Ingress)
		db := config.InitDB(true)

		importPlayerAdvanced(db)
		slog.Info("🎉 Player Advanced Import completed successfully")

		importPlayerAdvancedPlayoffs(db)
		slog.Info("🎉 Player Advanced Playoffs Import completed successfully")

		importPlayerTotalsScrape(db)
		slog.Info("🎉 Player Totals (scraped) Import completed successfully")

		importPlayerTotalsPlayoffsScrape(db)
		slog.Info("🎉 Player Playoffs (scraped) Import completed successfully")

		// importGameSchedules(db)
		// slog.Info("🎉 Game Imports completed successfully 🏀")

		// importBoxScores(db)
		// slog.Info("🎉 Related Box Score Imports completed successfully 📦")

		// importMarkPlayoffGames(db)
		// slog.Info("🎉 Playoff games marked successfully 🏆")

		importPlayerShotCharts(db)
		slog.Info("🎉 Player Shot Chart Import completed successfully 🎯")

		slog.Info("🏀 ALL Imports completed successfully ✅ 🙌")
		return
	}

	// ——— Normal API startup: skip AutoMigrate ———
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// — CORS Allow ALL origins (development) —
	app.Use(cors.New())

	// Middlewares
	app.Use(middleware.StructuredLogger())
	app.Use(middleware.RateLimiter())
	app.Use(middleware.MetricsMiddleware())

	// DB connection (Read/Write splitting via DBResolver)
	db := config.InitDB(false)

	/* ---------- HEALTH & OBSERVABILITY ROUTES ---------- */
	controllers.RegisterHealthRoutes(app, db)
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	/* ---------- PUBLIC ROUTES (no API key) ---------- */
	// Redirect root to swagger docs
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/swagger/index.html")
	})

	app.Get("/swagger/*", fiberswagger.WrapHandler)
	controllers.RegisterKeyAdminRoutes(app, db)

	/* ---------- PROTECTED ROUTES ---------- */
	// Remove Comment to re-enable api key middleware.
	// app.Use(middleware.APIKeyAuth(db))
	routes.RegisterPlayerAdvancedRoutes(app, db)
	routes.RegisterPlayerTotalRoutes(app, db)
	routes.RegisterPlayerShotChartRoutes(app, db)
	routes.RegisterGameRoutes(app, db)

	/* ---------- START & SHUTDOWN ---------- */
	go func() {
		slog.Info("🚀 Starting NBA_Go Fiber API Server on :5000")
		if err := app.Listen(":5000"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed to listen on :5000", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()
	slog.Info("🛑 Shutting down server gracefully...")
	_ = app.Shutdown()
	slog.Info("👋 Server shutdown complete. Bye!")
}
