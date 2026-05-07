package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func RateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        20,
		Expiration: 1 * time.Minute,
		Next: func(c *fiber.Ctx) bool {
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
