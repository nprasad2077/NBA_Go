package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nprasad2077/NBA_Go/utils/metrics"
)

// MetricsMiddleware tracks HTTP metrics for Prometheus
func MetricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Process request
		err := c.Next()
		
		// Record metrics after request is processed
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		path := c.Route().Path
		
		// Increment request counter
		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		
		// Record request duration
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
		
		return err
	}
}