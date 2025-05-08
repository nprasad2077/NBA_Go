// main_test.go
package main

import (
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/nprasad2077/NBA_Go/models"
	"github.com/nprasad2077/NBA_Go/routes"
	"github.com/nprasad2077/NBA_Go/utils/security"
)

// -----------------------------------------------------------------------------
// test bootstrap
// -----------------------------------------------------------------------------
func setupTestApp() (*fiber.App, string) {
	app := fiber.New()

	// in‑memory SQLite so tests don’t touch real file
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	_ = db.AutoMigrate(&models.PlayerAdvancedStat{}, &models.APIKey{})

	// seed one API key we can use in the requests
	rawKey := "testkey123"
	db.Create(&models.APIKey{Hash: security.HashKey(rawKey)})

	// register only the routes we need
	routes.RegisterPlayerAdvancedRoutes(app, db)

	return app, rawKey
}

// -----------------------------------------------------------------------------
// actual test
// -----------------------------------------------------------------------------
func TestGetPlayerAdvancedStats(t *testing.T) {
	app, key := setupTestApp()

	tests := []struct {
		name          string
		route         string
		wantCode      int
		wantSubstring string
	}{
		{
			name:          "valid route",
			route:         "/api/playeradvancedstats/",
			wantCode:      200,
			wantSubstring: `"data"`, // expect JSON payload has "data"
		},
		{
			name:          "invalid route",
			route:         "/api/invalid",
			wantCode:      404,
			wantSubstring: "Cannot GET /api/invalid",
		},
	}

	for _, tc := range tests {
		req, _ := http.NewRequest(http.MethodGet, tc.route, nil)
		req.Header.Set("X-API-Key", key)

		resp, err := app.Test(req, -1)
		assert.NoError(t, err, tc.name)
		assert.Equal(t, tc.wantCode, resp.StatusCode, tc.name)

		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), tc.wantSubstring, tc.name)
	}
}