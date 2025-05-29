// @title       NBA_Go API
// @version     1.0
// @description Stats service with API-key auth
// @schemes     http https
// @BasePath    /
//
// @securityDefinitions.apikey ApiKeyAuth
// @in   header
// @name X-API-Key
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	fiberswagger "github.com/swaggo/fiber-swagger"

	"github.com/nprasad2077/NBA_Go/config"
	"github.com/nprasad2077/NBA_Go/utils"
	"github.com/nprasad2077/NBA_Go/controllers"
	"github.com/nprasad2077/NBA_Go/routes"
	"github.com/nprasad2077/NBA_Go/utils/middleware"
	_ "github.com/nprasad2077/NBA_Go/docs"
)

func main() {
	// ——— One-off import-data mode ———
	if len(os.Args) > 1 && os.Args[1] == "import-data" {
		db := config.InitDB()

		importPlayerAdvanced(db)
		log.Println("🎉 Player Advanced Import completed successfully")
		utils.SleepWithJitter(1100 * time.Millisecond)

		importPlayerAdvancedPlayoffs(db)
		log.Println("🎉 Player Advanced Playoffs Import completed successfully")
		
		// importPlayerTotals(db)
		// log.Println("🎉 Player Totals Import completed successfully")
		// importPlayerPlayoffs(db)
		// log.Println("🎉 Player Totals Playoffs Import completed successfully")

		importPlayerTotalsScrape(db)
        log.Println("🎉 Player Totals (scraped) Import completed successfully")
        importPlayerTotalsPlayoffsScrape(db)
        log.Println("🎉 Player Playoffs (scraped) Import completed successfully")

		// importPlayerShotChart(db)
		// log.Println("🎉 Player Shot Chart Import Completed Successfully ")
		
		log.Println("🎉 Import completed successfully")
		return
	}

	// gracefull shutdown context
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

	// middlewares
	app.Use(logger.New())
	app.Use(middleware.MetricsMiddleware())

	// DB connection
	db := config.InitDB()

	/* ---------- PUBLIC ROUTES (no API key) ---------- */
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	app.Get("/swagger/*", fiberswagger.WrapHandler)
	controllers.RegisterKeyAdminRoutes(app, db)

	/* ---------- PROTECTED ROUTES ---------- */
	app.Use(middleware.APIKeyAuth(db))
	routes.RegisterPlayerAdvancedRoutes(app, db)
	routes.RegisterPlayerTotalRoutes(app, db)
	routes.RegisterPlayerShotChartRoutes(app, db)

	/* ---------- START & SHUTDOWN ---------- */
	go func() {
		if err := app.Listen(":5000"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Println("shutting down…")
	_ = app.Shutdown()
	log.Println("bye")
}