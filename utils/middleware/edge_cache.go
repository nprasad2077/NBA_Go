package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// CacheStrategy defines caching profiles for different API data mutability tiers
type CacheStrategy string

const (
	// Immutable: For historical completed games, past seasons, and shot charts.
	// Edge TTL: 30 Days | Browser TTL: 1 Day
	Immutable CacheStrategy = "immutable"

	// SemiDynamic: For daily leaderboards, player season averages, and standings.
	// Edge TTL: 1 Day | Browser TTL: 1 Minute | Stale-While-Revalidate: 10 Minutes
	SemiDynamic CacheStrategy = "semi_dynamic"

	// Realtime: For active game telemetry, live quarter stats, and fast-updating data.
	// Edge TTL: 10 Seconds | Browser TTL: 5 Seconds | Stale-While-Revalidate: 30 Seconds
	Realtime CacheStrategy = "realtime"

	// NoCache: For transactional mutations, scrapers, and admin endpoints.
	NoCache CacheStrategy = "no_cache"
)

// EdgeCache injects standardized Cloudflare & HTTP caching headers
func EdgeCache(strategy CacheStrategy, tags ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only cache idempotent read requests (GET / HEAD)
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Set("Pragma", "no-cache")
			c.Set("Expires", "0")
			return c.Next()
		}

		switch strategy {
		case Immutable:
			// Served directly by Cloudflare US Edge for 30 days without hitting Europe origin
			c.Set("Cache-Control", "public, max-age=86400, s-maxage=2592000, immutable")
		case SemiDynamic:
			// Instant edge delivery; background revalidation over transatlantic backbone
			c.Set("Cache-Control", "public, max-age=60, s-maxage=86400, stale-while-revalidate=600")
		case Realtime:
			c.Set("Cache-Control", "public, max-age=5, s-maxage=10, stale-while-revalidate=30")
		case NoCache:
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate")
			return c.Next()
		}

		// Cloudflare Cache-Tag header for instantaneous programmatic purge by entity
		if len(tags) > 0 {
			var tagHeader string
			for i, tag := range tags {
				if i > 0 {
					tagHeader += ","
				}
				tagHeader += tag
			}
			c.Set("Cache-Tag", tagHeader)
		}

		c.Set("Vary", "Accept-Encoding, Origin")
		return c.Next()
	}
}

// ComputeETag generates a weak ETag based on response body bytes for 304 Not Modified optimization
func ComputeETag(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(hash[:8]))
}
