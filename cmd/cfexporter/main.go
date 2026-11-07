// Command cfexporter polls Cloudflare's GraphQL Analytics API for edge cache
// statistics on a single hostname and exposes them as Prometheus counters.
//
// Cloudflare's analytics rate-limit budget is small on the Free plan, so
// Grafana must not query it directly on every dashboard refresh. Instead this
// exporter polls once per interval over non-overlapping time windows and adds
// each window's counts to monotonically increasing counters, which Prometheus
// scrapes and retains like any other metric.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultGraphQLURL = "https://api.cloudflare.com/client/v4/graphql"

// maxWindow caps how far back a single catch-up poll reaches. Adaptive
// analytics on the Free plan are only queryable for roughly the last 24h.
const maxWindow = 6 * time.Hour

type config struct {
	token    string
	zoneID   string
	host     string
	url      string
	listen   string
	interval time.Duration
	delay    time.Duration
}

type exporter struct {
	cfg     config
	client  *http.Client
	lastEnd time.Time

	requests    *prometheus.CounterVec
	bytes       *prometheus.CounterVec
	errors      prometheus.Counter
	lastSuccess prometheus.Gauge
}

func newExporter(cfg config, reg prometheus.Registerer) *exporter {
	e := &exporter{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cf_edge_requests_total",
			Help: "Requests served by the Cloudflare edge for the configured host, by cache status and path scope.",
		}, []string{"host", "scope", "cache_status"}),
		bytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cf_edge_response_bytes_total",
			Help: "Bytes served by the Cloudflare edge for the configured host, by cache status and path scope.",
		}, []string{"host", "scope", "cache_status"}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cf_exporter_poll_errors_total",
			Help: "Failed Cloudflare GraphQL polls.",
		}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cf_exporter_last_success_timestamp_seconds",
			Help: "Unix time of the last successful poll.",
		}),
	}
	reg.MustRegister(e.requests, e.bytes, e.errors, e.lastSuccess)
	return e
}

// window returns the next [start, end) range to query, or ok=false when the
// window would be empty. end is truncated to the minute and held back by the
// configured delay so Cloudflare has finished ingesting that minute.
func (e *exporter) window(now time.Time) (start, end time.Time, ok bool) {
	end = now.Add(-e.cfg.delay).Truncate(time.Minute)
	start = e.lastEnd
	if start.IsZero() {
		start = end.Add(-e.cfg.interval)
	}
	if end.Sub(start) > maxWindow {
		start = end.Add(-maxWindow)
	}
	return start, end, end.After(start)
}

const query = `query($zone: String!, $host: String!, $since: Time!, $until: Time!) {
  viewer { zones(filter: {zoneTag: $zone}) {
    api: httpRequestsAdaptiveGroups(limit: 100, filter: {
      datetime_geq: $since, datetime_lt: $until,
      clientRequestHTTPHost: $host, clientRequestPath_like: "/api/%"
    }) { count sum { edgeResponseBytes } dimensions { cacheStatus } }
    other: httpRequestsAdaptiveGroups(limit: 100, filter: {
      datetime_geq: $since, datetime_lt: $until,
      clientRequestHTTPHost: $host, clientRequestPath_notlike: "/api/%"
    }) { count sum { edgeResponseBytes } dimensions { cacheStatus } }
  } }
}`

type group struct {
	Count int64 `json:"count"`
	Sum   struct {
		EdgeResponseBytes int64 `json:"edgeResponseBytes"`
	} `json:"sum"`
	Dimensions struct {
		CacheStatus string `json:"cacheStatus"`
	} `json:"dimensions"`
}

type gqlResponse struct {
	Data struct {
		Viewer struct {
			Zones []struct {
				API   []group `json:"api"`
				Other []group `json:"other"`
			} `json:"zones"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (e *exporter) fetch(ctx context.Context, start, end time.Time) (*gqlResponse, error) {
	body, _ := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]string{
			"zone":  e.cfg.zoneID,
			"host":  e.cfg.host,
			"since": start.UTC().Format(time.RFC3339),
			"until": end.UTC().Format(time.RFC3339),
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.cfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql HTTP %d", resp.StatusCode)
	}
	var out gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}
	if len(out.Data.Viewer.Zones) == 0 {
		return nil, fmt.Errorf("zone %s not visible to token", e.cfg.zoneID)
	}
	return &out, nil
}

// poll queries the next window and, only on success, adds the results to the
// counters and advances lastEnd. A failed poll leaves lastEnd unchanged so the
// next poll covers the gap.
func (e *exporter) poll(ctx context.Context, now time.Time) error {
	start, end, ok := e.window(now)
	if !ok {
		return nil
	}
	res, err := e.fetch(ctx, start, end)
	if err != nil {
		e.errors.Inc()
		return err
	}
	zone := res.Data.Viewer.Zones[0]
	for scope, groups := range map[string][]group{"api": zone.API, "other": zone.Other} {
		for _, g := range groups {
			e.requests.WithLabelValues(e.cfg.host, scope, g.Dimensions.CacheStatus).Add(float64(g.Count))
			e.bytes.WithLabelValues(e.cfg.host, scope, g.Dimensions.CacheStatus).Add(float64(g.Sum.EdgeResponseBytes))
		}
	}
	e.lastEnd = end
	e.lastSuccess.Set(float64(now.Unix()))
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Fatalf("%s: %v", key, err)
		}
		return d
	}
	return def
}

func main() {
	cfg := config{
		token:    os.Getenv("CF_API_TOKEN"),
		zoneID:   os.Getenv("CF_ZONE_ID"),
		host:     env("CF_HOST", "nba.space-city.dev"),
		url:      env("CF_GRAPHQL_URL", defaultGraphQLURL),
		listen:   env("LISTEN_ADDR", ":9200"),
		interval: envDuration("POLL_INTERVAL", 5*time.Minute),
		delay:    envDuration("POLL_DELAY", 2*time.Minute),
	}
	if cfg.token == "" || cfg.zoneID == "" {
		log.Fatal("CF_API_TOKEN and CF_ZONE_ID are required")
	}

	reg := prometheus.NewRegistry()
	e := newExporter(cfg, reg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		t := time.NewTicker(cfg.interval)
		defer t.Stop()
		for {
			if err := e.poll(ctx, time.Now()); err != nil {
				log.Printf("poll failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: cfg.listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	log.Printf("cfexporter host=%s interval=%s delay=%s listening on %s", cfg.host, cfg.interval, cfg.delay, cfg.listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
