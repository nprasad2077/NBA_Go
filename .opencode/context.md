# Codebase Context — NBA_Go
Updated: 2026-10-21

## Overview
`NBA_Go` is a high-performance REST API and ETL ingestion service for historical and live NBA statistics built in Go (Fiber) and PostgreSQL. It scrapes game schedules, box scores, line scores, player totals, advanced analytics (PER, WS, VORP, BPM), and player shot chart coordinates from Basketball Reference, persists them with GORM using read/write splitting via HAProxy, and serves paginated and filtered queries behind NGINX with multi-tier edge/proxy caching, dual-stack IPv4/IPv6 networking, rate limiting, and Prometheus/Grafana observability.

## Stack
- **Language & Runtime**: Go (`go 1.23.0`, `toolchain go1.24.2` in `go.mod:3-5`; container runtime uses `golang:1.24-bullseye` builder and `debian:bookworm-slim` base in `Dockerfile:2,21`).
- **Web Framework**: Go Fiber v2 (`github.com/gofiber/fiber/v2 v2.52.6`).
- **Persistence & ORM**: GORM v1.25.12 (`gorm.io/gorm`), PostgreSQL driver v1.6.0 (`gorm.io/driver/postgres`), SQLite driver v1.5.7 (`gorm.io/driver/sqlite` for in-memory testing), DBResolver plugin v1.5.3 (`gorm.io/plugin/dbresolver`).
- **HTML Parsing & Scraping**: `github.com/PuerkitoBio/goquery v1.10.3`, `golang.org/x/net v0.40.0`.
- **Metrics & Observability**: Prometheus Client v1.22.0 (`github.com/prometheus/client_golang`), Prometheus server (`prom/prometheus:latest`), Grafana (`grafana/grafana:latest`).
- **Documentation**: Swagger / OpenAPI 2.0 via `github.com/swaggo/swag v1.16.4` and `github.com/swaggo/fiber-swagger v1.3.0`.
- **Reverse Proxy / Cache**: NGINX (`nginx:stable`) with 30-second in-memory `api_cache` zone and referer filtering.
- **Testing**: Go `testing` standard package with Testify assertions (`github.com/stretchr/testify v1.10.0`).
- **Package Management**: Go Modules (`go.mod`, `go.sum`).

## Layout
- `main.go`: Application entrypoint; forks between the one-off `import-data` CLI command and the live HTTP server.
- `import.go`: ETL batch scrapers orchestrating seasonal imports for advanced stats, totals, schedules, box scores, and shot charts.
- `cmd/`: Standalone CLI utilities (`cmd/generate_key.go` for cryptographic 32-byte API key generation).
- `config/`: Database configuration (`config/database.go`) managing env vars, primary/replica DSNs, connection pools, and GORM AutoMigrate.
- `controllers/`: HTTP handlers and response DTOs (`game_controller.go`, `player_total_controller.go`, `player_advanced_controller.go`, `player_shot_chart_controller.go`, `health_controller.go`, `api_key_controller.go`).
- `models/`: GORM database schema models (`game.go`, `player_total_stat.go`, `player_advanced_stat.go`, `player_shot_chart.go`, `api_key.go`).
- `routes/`: Route group registrations connecting Fiber endpoints to controllers (`game_routes.go`, `player_total_routes.go`, `player_advanced_routes.go`, `player_shot_chart_routes.go`).
- `services/`: Web scrapers extracting HTML tables from Basketball Reference and batch-upserting records (`player_total_scrape_service.go`, `player_advanced_playoffs_scrape_service.go`, `player_shot_chart_scrape_service.go`, `game_scrape_service.go`, `box_score_scrape_service.go`, `line_score_scrape_service.go`, `helpers.go`).
- `utils/`: Common utilities including rate limiting, edge cache headers, structured JSON logging, Prometheus metrics, API key hashing, sleep jitter, and NBA team abbreviations.
- `nginx/`: NGINX configuration (`nginx.conf`) and Dockerfile for reverse proxy load balancing and response caching.
- `prometheus/`: Prometheus configuration (`prometheus.yml`) scraping backend replicas and HAProxy.
- `grafana/`: Pre-provisioned dashboards (`dashboards/`) and data sources (`provisioning/`).
- `docs/`: OpenAPI specs (`swagger.json`, `swagger.yaml`), Postman collection, architectural overviews, and load testing recipes.
- `test/`: Test suites, load testing tool (`loadtest.go`), and benchmark logs (`results.log`).

## Architecture
- **Ingress & Reverse Proxy**: NGINX (`nginx/nginx.conf:22-63`) listens on container port `8080` (mapped to host `8081`), load balances across `api1:5000`, `api2:5000`, `api3:5000`, caches `/api/` GET responses for 30s (`nginx/nginx.conf:33-50`), blocks blacklisted referers (`nginx/nginx.conf:10-14,29-31`), and injects proxy headers (`X-Real-IP`, `X-Forwarded-For`).
- **HTTP Transport & Middleware**: Dual-stack listener on `[::]:5000` via `net.Listen("tcp", ":5000")` and `app.Listener` (`main.go:140-150`). Fiber middleware pipeline executes CORS (`main.go:102`), Brotli/Gzip compression (`main.go:105-107`), structured JSON logger (`utils/middleware/logger_middleware.go:15-39`), rate limiting (`utils/middleware/rate_limiter.go:11-20`), and Prometheus metrics instrumentation (`utils/middleware/metrics_middleware.go:14-29`).
- **Edge Cache Headers**: `utils/middleware/edge_cache.go:32-76` injects Cloudflare/HTTP caching headers based on data volatility: `Immutable` (Edge: 30 days, Browser: 1 day), `SemiDynamic` (Edge: 1 day, SWR: 10 mins), `Realtime` (Edge: 10s, SWR: 30s), and `NoCache`.
- **Database Routing (R/W Splitting)**: GORM uses `dbresolver` (`config/database.go:91-106`). Write queries route to HAProxy write ingress (`DB_WRITE_HOST:DB_WRITE_PORT`, defaults `178.105.149.129:5437` -> Primary DB). Read queries load-balance across HAProxy read ingress (`DB_READ_HOST:DB_READ_PORT`, defaults `178.105.149.129:5438` -> Read Replicas). Connection pool is tuned to 10 idle, 50 open conns, 30m idle timeout, 2h max lifetime.

## Entrypoints & flows
- **CLI / ETL Scraper Entrypoint**:
  - `go run . import-data` or `/nba_go import-data` triggers `main.go:52-83`.
  - Initializes DB with `config.InitDB(true)` (`config/database.go:69-138`), which auto-migrates 10 models to the Primary DB via `db.Clauses(dbresolver.Write).AutoMigrate(...)`.
  - Runs ETL sequences in `import.go:14-184`, iterating over seasons/players, fetching HTML from Basketball Reference via `services/` scrapers, parsing tables with `goquery`, and batch upserting into PostgreSQL with `clause.OnConflict{UpdateAll: true}` while inserting sleep jitter (`utils/sleep.go:10-18`) to avoid rate limits.
- **Representative API Read Flow (`GET /api/playertotals`)**:
  - `Client` requests `GET https://nba.turbo-data.com/api/playertotals?season=2024&pageSize=20&page=1`.
  - `NGINX` receives request; checks cache zone `api_cache:10m` (`nginx/nginx.conf:33-50`). If cache HIT, returns cached 200 immediately with `X-Cache-Status: HIT`.
  - If MISS, forwards to `go_backend` (`api1|api2|api3:5000`).
  - Fiber middleware applies compression, logger, rate limiter, and Prometheus request counter/histogram.
  - `middleware.EdgeCache` evaluates route strategy `SemiDynamic` (`routes/player_total_routes.go:16`) and sets `Cache-Control: public, max-age=60, s-maxage=86400, stale-while-revalidate=600`.
  - `controllers.GetPlayerTotalStats` parses query parameters (`controllers/player_total_controller.go:139-251`), builds dynamic GORM query (`Season`, `Team`, `PlayerID`, `IsPlayoff`, `SortBy`, `Ascending`).
  - GORM DBResolver executes `Count(&total)` and `Find(&stats)` over HAProxy read replica pool (`:5438`).
  - Controller maps database models to `PlayerTotalStatDTO` (omitting internal DB IDs and timestamps) and returns wrapped in `PlayerTotalsResponse` pagination envelope.

## Data & integrations
- **Databases**: PostgreSQL 15+ cluster accessed via HAProxy (Write: `:5437`, Read: `:5438`). Local environment uses `postgres:15-alpine` container (`docker-compose.local.yml:5-21`).
- **Core Models**: `APIKey`, `PlayerAdvancedStat`, `PlayerTotalStat`, `PlayerShotChart`, `Game`, `LineScore`, `PlayerGameBasicStat`, `PlayerGameAdvStat`, `TeamGameBasicStat`, `TeamGameAdvStat` (`models/*.go`).
- **External Services**: Basketball-Reference.com (`https://www.basketball-reference.com/`) for HTML scraping of schedules, box scores, player season totals, advanced analytics, and shot coordinates.
- **Auth Model**:
  - Public endpoints: Open access currently enabled; `middleware.APIKeyAuth` is commented out in `main.go:132`.
  - API Key system: Header `X-API-Key` hashed with SHA-256 and checked against `api_keys` table (`utils/security/api_key.go:12-25`, `utils/middleware/api_key_middleware.go:13-37`).
  - Key Admin endpoints: `/admin/keys` protected by `X-Admin-Secret` header matching `$ADMIN_SECRET` (`controllers/api_key_controller.go:15-23`).

## Commands
- **Run API Server**: `go run .` or `./NBA_Go`
- **Run ETL Scraper**: `go run . import-data`
- **Run Tests**: `go test -v ./...`
- **Run Specific Test**: `go test -v -run TestGetPlayerAdvancedStats .`
- **Build Binary**: `go build -o NBA_Go .`
- **Generate API Key**: `go run cmd/generate_key.go`
- **Start Local Docker Stack**: `make up` (or `docker-compose -f docker-compose.local.yml up --build -d`)
- **Stop Local Docker Stack**: `make down` (or `docker-compose down`)
- **Regenerate Swagger Docs**: `swag init`

## Conventions
- **Code Style & Formatting**: Standard `gofmt` / `goimports`. Standard library `log/slog` structured JSON logging (`main.go:49-50`).
- **Layering & Separation**: Strict separation between `routes/` (URL definitions and cache middleware), `controllers/` (query parsing, DTO transformation, HTTP responses), `services/` (scraping, HTML parsing, GORM batch upserts), and `models/` (GORM entity definitions).
- **Database Upserts**: Scrapers use GORM `clause.OnConflict` with composite unique indexes (e.g. `idx_total_player_season_team`) to ensure idempotent re-scrapes without duplicate records.
- **Branching & Commits**: Git branch `shooting`; commits follow conventional commit prefixes (`feat:`, `fix:`, `docs:`).

## Testing
- **Framework**: Go `testing` with Testify `assert` (`github.com/stretchr/testify/assert`).
- **Test Locations**:
  - `main_test.go`: Controller and route integration tests using in-memory SQLite (`gorm.io/driver/sqlite`) and Fiber's `app.Test()`.
  - `test/loadtest_test.go`: Unit tests for load testing helpers (request generators, cache extractors, page mix weight distributions, percentile calculations).
- **Load Testing**:
  - `test/loadtest.go`: High-concurrency benchmark tool supporting request rates, target URLs, percentile calculations, and CDN cache HIT/MISS ratio reporting.

## Open questions
- **API Key Enforcement**: API key authentication middleware is commented out (`main.go:132`). Confirm if production is intentionally public or if API key verification will be re-enabled.
- **Import Target Hardcoding**: `import.go:102-153` contains hardcoded seasons (2017–2026) and a specific slice of player IDs (`brunsja01`, `edwaran01`, etc.) for shot chart scraping. Confirm if dynamic CLI arguments or configuration flags are planned for full roster ingestion.
- **Dirty Log File**: `test/results.log` contains 590k+ uncommitted lines from load test runs. Confirm if `test/*.log` should be added to `.gitignore`.

## Starting points
- **Adding / Modifying Endpoints**: Start in `routes/` to define the path and edge cache tier, then implement business logic and DTO response shapes in `controllers/`.
- **Scraper / ETL Adjustments**: Start in `services/` for HTML selectors and data parsing; configure season batch ranges and rate limiting in `import.go`.
- **Database Schema Changes**: Add/modify fields in `models/`, add model pointer to `modelsToMigrate` in `config/database.go:115-126`, and trigger migration via `go run . import-data`.
- **Performance Tuning & Caching**: Check `utils/middleware/edge_cache.go` for cache control headers and `nginx/nginx.conf` for reverse proxy cache keys and invalidation policies.
