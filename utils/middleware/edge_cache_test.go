package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestEdgeCacheMiddleware(t *testing.T) {
	tests := []struct {
		name              string
		method            string
		strategy          CacheStrategy
		tags              []string
		expectedCacheCtrl string
		expectedCacheTag  string
	}{
		{
			name:              "SemiDynamic strategy for games with tag",
			method:            fiber.MethodGet,
			strategy:          SemiDynamic,
			tags:              []string{"nba-games"},
			expectedCacheCtrl: "public, max-age=60, s-maxage=86400, stale-while-revalidate=600",
			expectedCacheTag:  "nba-games",
		},
		{
			name:              "Immutable strategy",
			method:            fiber.MethodGet,
			strategy:          Immutable,
			tags:              []string{"nba-games", "nba-history"},
			expectedCacheCtrl: "public, max-age=86400, s-maxage=2592000, immutable",
			expectedCacheTag:  "nba-games,nba-history",
		},
		{
			name:              "Realtime strategy",
			method:            fiber.MethodGet,
			strategy:          Realtime,
			tags:              nil,
			expectedCacheCtrl: "public, max-age=5, s-maxage=10, stale-while-revalidate=30",
			expectedCacheTag:  "",
		},
		{
			name:              "NoCache strategy",
			method:            fiber.MethodGet,
			strategy:          NoCache,
			tags:              nil,
			expectedCacheCtrl: "no-store, no-cache, must-revalidate",
			expectedCacheTag:  "",
		},
		{
			name:              "Non-idempotent method POST should not cache",
			method:            fiber.MethodPost,
			strategy:          SemiDynamic,
			tags:              []string{"nba-games"},
			expectedCacheCtrl: "no-store, no-cache, must-revalidate, proxy-revalidate",
			expectedCacheTag:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			handler := EdgeCache(tc.strategy, tc.tags...)

			app.Add(tc.method, "/test", handler, func(c *fiber.Ctx) error {
				return c.SendString("ok")
			})

			req := httptest.NewRequest(tc.method, "/test", nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			assert.Equal(t, tc.expectedCacheCtrl, resp.Header.Get("Cache-Control"))
			if tc.expectedCacheTag != "" {
				assert.Equal(t, tc.expectedCacheTag, resp.Header.Get("Cache-Tag"))
			} else {
				assert.Empty(t, resp.Header.Get("Cache-Tag"))
			}
		})
	}
}
