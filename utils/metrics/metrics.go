package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Define metrics
var (
	// HTTPRequestsTotal counts the number of HTTP requests processed
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nba_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// HTTPRequestDuration measures the duration of HTTP requests
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nba_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// DBOperationsTotal counts database operations
	DBOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nba_db_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "entity"},
	)
)