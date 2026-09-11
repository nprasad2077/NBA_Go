package middleware

import (
	"crypto/subtle"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func RateLimiter() fiber.Handler {
	benchmarkKey := strings.TrimSpace(os.Getenv("BENCHMARK_KEY"))

	maxRequests := 20
	if envMax := os.Getenv("RATE_LIMIT_MAX"); envMax != "" {
		if val, err := strconv.Atoi(envMax); err == nil && val > 0 {
			maxRequests = val
		}
	}

	expiration := 1 * time.Minute
	if envExp := os.Getenv("RATE_LIMIT_EXPIRATION_SECONDS"); envExp != "" {
		if val, err := strconv.Atoi(envExp); err == nil && val > 0 {
			expiration = time.Duration(val) * time.Second
		}
	}

	return limiter.New(limiter.Config{
		Max:        maxRequests,
		Expiration: expiration,
		Next: func(c *fiber.Ctx) bool {
			// Bypass rate limiting when a valid internal benchmark key is supplied
			if benchmarkKey != "" {
				clientKey := c.Get("X-Benchmark-Key")
				if clientKey != "" && subtle.ConstantTimeCompare([]byte(clientKey), []byte(benchmarkKey)) == 1 {
					return true
				}
			}

			// Skip rate limiting for internal services and infra endpoints
			ip := c.IP()
			if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") || ip == "127.0.0.1" {
				return true
			}
			path := c.Path()
			return path == "/metrics" || strings.HasPrefix(path, "/swagger")
		},
		KeyGenerator: func(c *fiber.Ctx) string {
			if ip := c.Get("X-Real-IP"); ip != "" {
				return ip
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Try again later.",
			})
		},
	})
}
