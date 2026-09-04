package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// StructuredLogger logs incoming HTTP requests using slog with X-Request-ID correlation.
func StructuredLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
			c.Set("X-Request-ID", reqID)
		}
		c.Locals("requestId", reqID)

		err := c.Next()
		duration := time.Since(start)

		slog.Info("HTTP Request",
			"request_id", reqID,
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", duration.Milliseconds(),
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		)
		return err
	}
}
