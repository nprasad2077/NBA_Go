# Codebase Context — NBA_Go
Updated: 2026-11-06

## Overview
`NBA_Go` is a high-performance REST API, ETL ingestion engine, and observability suite for historical and live NBA statistics built in Go (Fiber v2) and PostgreSQL. It scrapes game schedules, box scores, line scores, player totals, advanced analytics (PER, WS, VORP, BPM), and player shot chart coordinates from Basketball Reference, persists them with GORM using read/write splitting via HAProxy, and serves paginated/filtered queries behind NGINX with multi-tier edge caching headers, dual-stack IPv4/IPv6 networking, rate limiting, and an observability pipeline featuring Prometheus, Grafana, an HAProxy exporter, an NGINX syslog log exporter, and a Cloudflare GraphQL edge metrics exporter.

## Stack
- **Language & Runtime**: Go (`go 1.23.0`, `toolchain go1.24.2` in `go.mod:3,5`; container builder uses `golang:1.24-bullseye` and runtime base `debian:bookworm-slim` in `Dockerfile:2,21`).
- **Web Framework**: Go Fiber v2 (`github.com/gofiber/fiber/v2 v2.52.6`).
- **Persistence & ORM**: GORM v1.25.12 (`gorm.io/gorm`), PostgreSQL driver v1.6.0 (`gorm.io/driver/postgres`), SQLite driver v1.5.7 (`gorm.io/driver/sqlite` for in-memory testing), DBResolver plugin v1.5.3 (`gorm.io/plugin/dbresolver`).
- **HTML Scraping & Parsing**: `github.com/PuerkitoBio/goquery v1.10.3`, `golang.org/x/net v0.40.0`.
- **Reverse Proxy & Ingress**: NGINX (`nginx:stable`) with `proxy_cache api_cache` (10m keys zone, 100m max size, 30s cache TTL) and referer blacklisting.
- **Metrics & Observability**:
  - Prometheus Client v1.22.0 (`github.com/prometheus/client_golang`).
  - Prometheus Server (`prom/prometheus:latest`).
  - Grafana (`grafana/grafana:latest`).
  - HAProxy Exporter v0.15.0 (`prom/haproxy-exporter:v0.15.0`).
  - NGINX Syslog Log Exporter (`quay.io/martinhelmich/prometheus-nginxlog-exporter:v1.11.0` via `nginx/Dockerfile.log-exporter`).
  - Cloudflare Edge Analytics Exporter (`cmd/cfexporter/main.go`).
- **Documentation**: Swagger / OpenAPI 2.0 via `github.com/swaggo/swag v1.16.4` and `github.com/swaggo/fiber-swagger v1.3.0`.
- **Testing**: Go `testing` standard package with Testify assertions (`github.com/stretchr/testify v1.10.0`).
- **Package Management**: Go Modules (`go.mod`, `go.sum`).

## Layout
- `main.go`: Application entrypoint; switches between CLI `import-data` ETL mode and HTTP API server; binds dual-stack `[::]:5000`.
- `import.go`: Scraper orchestration workflows for historical and seasonal stats, box scores, schedules, and shot charts.
- `cmd/`: Standalone CLI utilities:
  - `cmd/generate_key.go`: Generates 32-byte cryptographic random API keys.
  - `cmd/cfexporter/`: Cloudflare GraphQL Analytics poller exposing Prometheus edge cache metrics.
- `config/`: Database configuration (`config/database.go`) managing DSNs, connection pool tuning, HAProxy R/W splitting, and GORM AutoMigrate.
- `controllers/`: HTTP handlers, request validation, and response DTO mapping (`game_controller.go`, `player_total_controller.go`, `player_advanced_controller.go`, `player_shot_chart_controller.go`, `health_controller.go`, `api_key_controller.go`).
- `models/`: GORM database schema definitions (`game.go`, `player_total_stat.go`, `player_advanced_stat.go`, `player_shot_chart.go`, `api_key.go`).
- `routes/`: Fiber endpoint registrations and edge caching middleware tiers (`game_routes.go`, `player_total_routes.go`, `player_advanced_routes.go`, `player_shot_chart_routes.go`).
- `services/`: Scrapers fetching and parsing HTML from Basketball Reference (`player_total_scrape_service.go`, `player_advanced_playoffs_scrape_service.go`, `player_shot_chart_scrape_service.go`, `game_scrape_service.go`, `box_score_scrape_service.go`, `line_score_scrape_service.go`, `helpers.go`).
- `utils/`: Reusable helpers including rate limiting (`utils/middleware/rate_limiter.go`), edge cache headers (`utils/middleware/edge_cache.go`), JSON logging (`utils/middleware/logger_middleware.go`), Prometheus metrics (`utils/metrics/metrics.go`, `utils/middleware/metrics_middleware.go`), API key hashing (`utils/security/api_key.go`), jittered sleeps (`utils/sleep.go`), and team abbreviations (`utils/teams.go`).
- `nginx/`: NGINX reverse proxy configuration (`nginx.conf`), custom log exporter Dockerfile (`Dockerfile.log-exporter`), and Prometheus log-exporter config (`nginxlog-exporter.yml`).
- `prometheus/`: Prometheus configuration (`prometheus.yml`) scraping backend instances, HAProxy, NGINX cache metrics, and Cloudflare edge exporter.
- `grafana/`: Pre-configured dashboards (`dashboards/`) and automated datasource provisioning (`provisioning/`).
- `docs/`: OpenAPI/Swagger specs, Postman collections, architectural notes, and v1 redesign spec (`docs/superpowers/specs/2026-11-04-api-v1-query-redesign-design.md`).
- `test/`: Integration test suites, load generator (`test/loadtest.go`), unit tests (`test/loadtest_test.go`), and benchmark logs (`test/results.log`).

## Architecture
- **Ingress & Reverse Proxy**: NGINX (`nginx/nginx.conf:27-69`) listens on container `8080` (host `8081`), load balances across 3 API replicas (`api1`, `api2`, `api3` on port 5000), caches `/api/` 200 responses for 30s using `proxy_cache api_cache` (`nginx/nginx.conf:45-51`), filters blacklisted referers (`nginx/nginx.conf:15-19,34-36`), streams request metrics to `nginx-log-exporter:5531` via UDP syslog (`nginx/nginx.conf:43-44`), and passes upstream cache status (`X-Cache-Status`).
- **HTTP Transport & Middleware**: Fiber server binds dual-stack `[::]:5000` via `net.Listen("tcp", ":5000")` (`main.go:140-150`). Middleware order: CORS (`main.go:102`), Brotli/Gzip compression (`main.go:105-107`), structured JSON logger (`main.go:110`), rate limiter with `X-Benchmark-Key` bypass (`main.go:111`, `utils/middleware/rate_limiter.go:14-50`), and Prometheus metrics instrumentation (`main.go:112`).
- **Edge Cache Control**: `utils/middleware/edge_cache.go:32-76` manages downstream CDN headers based on data volatility: `Immutable` (`s-maxage=2592000`, 30 days), `SemiDynamic` (`s-maxage=86400`, `stale-while-revalidate=600`), `Realtime` (`s-maxage=10`, `stale-while-revalidate=30`), and `NoCache`.
- **Database Routing (R/W Splitting)**: GORM uses `dbresolver` (`config/database.go:91-106`). Write queries route to HAProxy write ingress (`DB_WRITE_HOST:DB_WRITE_PORT`, defaults `178.105.149.129:5437` -> Primary DB). Read queries load-balance across HAProxy read ingress (`DB_READ_HOST:DB_READ_PORT`, defaults `178.105.149.129:5438` -> Read Replicas). Connection pool is tuned to 10 idle, 50 open conns, 30m idle timeout, 2h max lifetime.
- **Health & Readiness Probes**: `/health/live` (and legacy `/healthz`) verifies container responsiveness; `/health/ready` (and legacy `/readyz`) executes active connection pings with 2s timeouts against both write (:5437) and read (:5438) database targets (`controllers/health_controller.go:28-124`).
- **Observability Pipeline**: Prometheus pulls metrics from 4 targets: backend replicas (`:5000/metrics`), HAProxy exporter (`:9101`), NGINX syslog exporter (`:4040`), and Cloudflare edge exporter (`:9200`) (`prometheus/prometheus.yml:8-29`).

## Entrypoints & flows
- **CLI / ETL Scraper Entrypoint**:
  - `go run . import-data` or `/nba_go import-data` triggers `main.go:52-83`.
  - Initializes DB with `config.InitDB(true)` (`config/database.go:69-138`), which auto-migrates 10 models to the Primary DB via `db.Clauses(dbresolver.Write).AutoMigrate(...)`.
  - Runs ETL sequences in `import.go:14-184`, iterating over seasons/players, fetching HTML from Basketball Reference via `services/` scrapers, parsing tables with `goquery`, and batch upserting into PostgreSQL with `clause.OnConflict{UpdateAll: true}` while inserting sleep jitter (`utils/sleep.go:10-18`) to avoid rate limits.
- **Representative API Read Flow (`GET /api/playertotals`)**:
  - `Client` requests `GET https://nba.space-city.dev/api/playertotals?season=2024&pageSize=20&page=1`.
  - `NGINX` receives request; checks cache zone `api_cache:10m` (`nginx/nginx.conf:45-51`). If cache HIT, returns cached 200 immediately with `X-Cache-Status: HIT`.
  - NGINX sends access log asynchronously over UDP syslog to `nginx-log-exporter:5531` (`nginx/nginx.conf:44`).
  - If MISS, forwards to `go_backend` (`api1|api2|api3:5000`).
  - Fiber middleware applies compression, logger, rate limiter, and Prometheus request counter/histogram.
  - `middleware.EdgeCache` evaluates route strategy `SemiDynamic` (`routes/player_total_routes.go:16`) and sets `Cache-Control: public, max-age=60, s-maxage=86400, stale-while-revalidate=600`.
  - `controllers.GetPlayerTotalStats` parses query parameters (`controllers/player_total_controller.go:139-251`), builds dynamic GORM query (`Season`, `Team`, `PlayerID`, `IsPlayoff`, `SortBy`, `Ascending`).
  - GORM DBResolver executes `Count(&total)` and `Find(&stats)` over HAProxy read replica pool (`:5438`).
  - Controller maps database models to `PlayerTotalStatDTO` (omitting internal DB IDs and timestamps) and returns wrapped in `PlayerTotalsResponse` pagination envelope.
- **Cloudflare Edge Exporter Flow (`cmd/cfexporter`)**:
  - `cmd/cfexporter/main.go` runs every 5 minutes (`POLL_INTERVAL`), querying Cloudflare GraphQL Analytics API (`cmd/cfexporter/main.go:27`) for `nba.space-city.dev` edge requests and bytes.
  - Accumulates non-overlapping window deltas into monotonically increasing Prometheus counters `cf_edge_requests_total` and `cf_edge_bytes_total` served on `:9200/metrics`.

## Data & integrations
- **Databases**: PostgreSQL 15+ cluster accessed via HAProxy (Write: `:5437`, Read: `:5438`). Local environment uses `postgres:15-alpine` container (`docker-compose.local.yml:5-21`).
- **Core Models**: `APIKey`, `PlayerAdvancedStat`, `PlayerTotalStat`, `PlayerShotChart`, `Game`, `LineScore`, `PlayerGameBasicStat`, `PlayerGameAdvStat`, `TeamGameBasicStat`, `TeamGameAdvStat` (`models/*.go`).
- **External Services**: Basketball-Reference.com (`https://www.basketball-reference.com/`) for HTML scraping of schedules, box scores, player season totals, advanced analytics, and shot coordinates.
- **External APIs**: Cloudflare GraphQL Analytics API (`https://api.cloudflare.com/client/v4/graphql`) for zone edge metrics (`cmd/cfexporter/main.go:27`).
- **Auth Model**:
  - Public endpoints: Open access currently enabled; `middleware.APIKeyAuth` is commented out in `main.go:132`.
  - API Key system: Header `X-API-Key` hashed with SHA-256 and checked against `api_keys` table (`utils/security/api_key.go:12-25`, `utils/middleware/api_key_middleware.go:13-37`).
  - Key Admin endpoints: `/admin/keys` protected by `X-Admin-Secret` header matching `$ADMIN_SECRET` (`controllers/api_key_controller.go:15-23`).
  - Benchmark bypass: Header `X-Benchmark-Key` matching `$BENCHMARK_KEY` bypasses rate limits (`utils/middleware/rate_limiter.go:37-41`).

## Commands
- **Run API Server**: `go run .` or `./NBA_Go`
- **Run ETL Scraper**: `go run . import-data`
- **Run Tests**: `go test -v ./...`
- **Run Specific Test**: `go test -v -run TestGetPlayerAdvancedStats .`
- **Build Binary**: `go build -o NBA_Go .`
- **Generate API Key**: `go run cmd/generate_key.go`
- **Run Cloudflare Exporter**: `go run cmd/cfexporter/main.go`
- **Run Load Test**: `go run test/loadtest.go -url http://localhost:8081/api/playertotals -c 10 -n 100`
- **Start Local Docker Stack**: `make up` (or `docker-compose -f docker-compose.local.yml up --build -d`)
- **Stop Local Docker Stack**: `make down` (or `docker-compose down`)
- **Regenerate Swagger Docs**: `swag init`

## Conventions
- **Code Style & Formatting**: Standard `gofmt` / `goimports`. Standard library `log/slog` structured JSON logging (`main.go:49-50`).
- **Layering & Separation**: Strict separation between `routes/` (URL definitions and cache middleware), `controllers/` (query parsing, DTO transformation, HTTP responses), `services/` (scraping, HTML parsing, GORM batch upserts), and `models/` (GORM entity definitions).
- **Database Upserts**: Scrapers use GORM `clause.OnConflict` with composite unique indexes (e.g. `idx_total_player_season_team`) to ensure idempotent re-scrapes without duplicate records.
- **Branching & Commits**: Git branch `grafana`; commits follow conventional commit prefixes (`feat:`, `fix:`, `docs:`).

## Testing
- **Framework**: Go `testing` with Testify `assert` (`github.com/stretchr/testify/assert`).
- **Test Locations**:
  - `main_test.go`: Controller and route integration tests using in-memory SQLite (`gorm.io/driver/sqlite`) and Fiber's `app.Test()`.
  - `cmd/cfexporter/main_test.go`: Cloudflare exporter unit tests using mock HTTP test servers and Prometheus `testutil` metric assertions.
  - `test/loadtest_test.go`: Unit tests for load testing helpers (request generators, cache extractors, page mix weight distributions, percentile calculations).
- **Load Testing**:
  - `test/loadtest.go`: High-concurrency benchmark tool supporting request rates, target URLs, percentile calculations, and CDN cache HIT/MISS ratio reporting.

## Open questions
- **API Key Enforcement**: API key authentication middleware is commented out (`main.go:132`). Confirm if production is intentionally public or if API key verification will be re-enabled under `/v1`.
- **API v1 Migration Timeline**: `docs/superpowers/specs/2026-11-04-api-v1-query-redesign-design.md` specifies a comprehensive v1 query redesign (player search, unified filters, standardized pagination). Confirm when implementation begins.
- **Import Target Hardcoding**: `import.go:102-153` contains hardcoded seasons (2017–2026) and a specific slice of player IDs (`brunsja01`, `edwaran01`, etc.) for shot chart scraping. Confirm if dynamic CLI arguments or configuration flags are planned for full roster ingestion.
- **Uncommitted Load Test Log**: `test/results.log` is a ~131MB untracked log file. Confirm if `test/*.log` should be added to `.gitignore`.

## Starting points
- **Adding / Modifying Endpoints**: Start in `routes/` to define the path and edge cache tier, then implement business logic and DTO response shapes in `controllers/`.
- **Scraper / ETL Adjustments**: Start in `services/` for HTML selectors and data parsing; configure season batch ranges and rate limiting in `import.go`.
- **Database Schema Changes**: Add/modify fields in `models/`, add model pointer to `modelsToMigrate` in `config/database.go:115-126`, and trigger migration via `go run . import-data`.
- **Performance Tuning & Caching**: Check `utils/middleware/edge_cache.go` for cache control headers and `nginx/nginx.conf` for reverse proxy cache keys and invalidation policies.
- **Observability & Metrics**: Inspect `cmd/cfexporter/` for edge metrics, `nginx/nginxlog-exporter.yml` for NGINX cache hits/misses, and `prometheus/prometheus.yml` for scrape configurations.
