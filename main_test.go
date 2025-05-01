package main

import (
	"io"
	"net/http"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/routes"
	"github.com/nprasad2077/NBA_Go/config"
)

func setupTestApp() *fiber.App {
	app := fiber.New()
	db := config.InitDB() // uses SQLite file; can be mocked or in-memory for advanced testing
	routes.RegisterPlayerAdvancedRoutes(app, db)
	return app
}

func TestGetPlayerAdvancedStats(t *testing.T) {
	app := setupTestApp()

	tests := []struct {
		description   string
		route         string
		expectedCode  int
		expectInBody  string
	}{
		{
			description:  "valid route",
			route:        "/api/playeradvancedstats",
			expectedCode: 200,
			expectInBody: `"data"`, // assuming a JSON object with "data"
		},
		{
			description:  "invalid route",
			route:        "/api/invalid",
			expectedCode: 404,
			expectInBody: "Cannot GET /api/invalid",
		},
	}

	for _, tc := range tests {
		req, _ := http.NewRequest("GET", tc.route, nil)
		resp, err := app.Test(req, -1)
		assert.Nil(t, err, tc.description)
		assert.Equal(t, tc.expectedCode, resp.StatusCode, tc.description)

		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), tc.expectInBody, tc.description)
	}
}