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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/nprasad2077/NBA_Go/config"
    // "github.com/nprasad2077/NBA_Go/services"
	"github.com/nprasad2077/NBA_Go/controllers"
	"github.com/nprasad2077/NBA_Go/routes"
	"github.com/nprasad2077/NBA_Go/utils/middleware"
	_ "github.com/nprasad2077/NBA_Go/docs"
	fiberswagger "github.com/swaggo/fiber-swagger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
    "net/http" 
)

func main() {
	// graceful shutdown context
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

	// DB
	db := config.InitDB()

	/* ---------- PUBLIC ROUTES (no API key) ---------- */
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	app.Get("/swagger/*", fiberswagger.WrapHandler)
	controllers.RegisterKeyAdminRoutes(app, db) // guarded only by X‑Admin‑Secret

	/* ---------- PROTECTED ROUTES ---------- */
	app.Use(middleware.APIKeyAuth(db))
	routes.RegisterPlayerAdvancedRoutes(app, db)
	routes.RegisterPlayerTotalRoutes(app, db)

	/* ---------- Optional import job ---------- */
	// go func() {
    //     for season := 2023; season <= 2025; season++ {
    //         if err := services.FetchAndStorePlayerAdvancedStats(db, season); err != nil {
    //             log.Printf("Fetch failed for player advanced season %d: %v\n", season, err)
    //         } else {
    //             log.Printf("Fetch successful for player advanced season %d\n", season)
    //         }
    //         time.Sleep(1100 * time.Millisecond) // optional delay
    //     }
    //     log.Printf("player advanced Import Success")
    // }()

    // go func() {
    //     for season := 2023; season <= 2025; season++ {
    //         if err := services.FetchAndStorePlayerTotalStats(db, season); err != nil {
    //             log.Printf("Fetch failed for player totals season %d: %v\n", season, err)
    //         } else {
    //             log.Printf("Fetch successful for player totals season %d\n", season)
    //         }
    //         time.Sleep(1000 * time.Millisecond) // optional delay
    //     }
    //     log.Printf("player totals Import Success")
    // }()

	/* ---------- START & SHUTDOWN ---------- */
	go func() {
		if err := app.Listen(":5000"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()           // wait for SIGTERM/CTRL‑C
	stop()                 // stop receiving more signals
	log.Println("shutting down…")
	_ = app.Shutdown()     // stop accepting new conns
	log.Println("bye")
}
