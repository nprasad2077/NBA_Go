package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Define metrics
var (
	// HTTPRequestsTotal counts the number of HTTP requests processed
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nba_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "endpoint", "status"},
	)

	// HTTPRequestDuration measures the duration of HTTP requests
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nba_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	// DBOperationsTotal counts database operations (connect, migrate)
	DBOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nba_db_operations_total",
			Help: "Total number of database lifecycle operations",
		},
		[]string{"operation", "entity"},
	)

	// DBPoolOpenConns tracks current open connections per pool
	DBPoolOpenConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nba_db_pool_open_connections",
			Help: "Current number of open connections in pool",
		},
		[]string{"pool"},
	)

	// DBPoolInUseConns tracks connections currently executing queries
	DBPoolInUseConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nba_db_pool_in_use_connections",
			Help: "Current number of in-use active connections executing queries",
		},
		[]string{"pool"},
	)

	// DBPoolIdleConns tracks idle connections available in pool
	DBPoolIdleConns = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nba_db_pool_idle_connections",
			Help: "Current number of idle connections in pool",
		},
		[]string{"pool"},
	)

	// DBPoolWaitCount tracks connection wait occurrences
	DBPoolWaitCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nba_db_pool_wait_count_total",
			Help: "Total number of connection waits due to pool saturation",
		},
		[]string{"pool"},
	)

	// DBPoolWaitDuration tracks time spent waiting for a connection
	DBPoolWaitDuration = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nba_db_pool_wait_duration_seconds_total",
			Help: "Total seconds blocked waiting for a connection",
		},
		[]string{"pool"},
	)
)

// StartDBPoolMetricsCollector periodically collects sql.DBStats for both pools
func StartDBPoolMetricsCollector(db *gorm.DB) {
	if db == nil {
		return
	}

	// Initialize series with 0 so Prometheus sees the metric immediately
	DBPoolWaitCount.WithLabelValues("write").Add(0)
	DBPoolWaitCount.WithLabelValues("read").Add(0)
	DBPoolWaitDuration.WithLabelValues("write").Add(0)
	DBPoolWaitDuration.WithLabelValues("read").Add(0)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var prevWriteWaitCount, prevReadWaitCount int64
		var prevWriteWaitDuration, prevReadWaitDuration time.Duration

		for range ticker.C {
			// Write Pool (Sources)
			if writeDB, err := db.Clauses(dbresolver.Write).DB(); err == nil && writeDB != nil {
				stats := writeDB.Stats()
				DBPoolOpenConns.WithLabelValues("write").Set(float64(stats.OpenConnections))
				DBPoolInUseConns.WithLabelValues("write").Set(float64(stats.InUse))
				DBPoolIdleConns.WithLabelValues("write").Set(float64(stats.Idle))

				if delta := stats.WaitCount - prevWriteWaitCount; delta > 0 {
					DBPoolWaitCount.WithLabelValues("write").Add(float64(delta))
					prevWriteWaitCount = stats.WaitCount
				}
				if deltaDur := stats.WaitDuration - prevWriteWaitDuration; deltaDur > 0 {
					DBPoolWaitDuration.WithLabelValues("write").Add(deltaDur.Seconds())
					prevWriteWaitDuration = stats.WaitDuration
				}
			}

			// Read Pool (Replicas)
			if readDB, err := db.Clauses(dbresolver.Read).DB(); err == nil && readDB != nil {
				stats := readDB.Stats()
				DBPoolOpenConns.WithLabelValues("read").Set(float64(stats.OpenConnections))
				DBPoolInUseConns.WithLabelValues("read").Set(float64(stats.InUse))
				DBPoolIdleConns.WithLabelValues("read").Set(float64(stats.Idle))

				if delta := stats.WaitCount - prevReadWaitCount; delta > 0 {
					DBPoolWaitCount.WithLabelValues("read").Add(float64(delta))
					prevReadWaitCount = stats.WaitCount
				}
				if deltaDur := stats.WaitDuration - prevReadWaitDuration; deltaDur > 0 {
					DBPoolWaitDuration.WithLabelValues("read").Add(deltaDur.Seconds())
					prevReadWaitDuration = stats.WaitDuration
				}
			}
		}
	}()
}