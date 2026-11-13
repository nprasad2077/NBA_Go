# Codebase Context — NBA_Go

Root: /Volumes/ROG_BLACK/code/review/NBA_Go  
Git root: /Volumes/ROG_BLACK/code/review/NBA_Go  
Commit: f7ae315  
Branch: shooting  
Updated: 2026-10-21

> _Note: Currently running in **Plan mode**. Per plan mode rules, file modifications outside `/Users/ravi/.opencode/plan` are restricted. The full verified scan is reported below._

---

## Overview

`NBA_Go` is a high-performance REST API and ETL service for NBA historical and regular-season statistics built with Go (Fiber) and PostgreSQL. It scrapes game schedules, box scores, line scores, player totals, advanced analytics (PER, WS, VORP, BPM), and player shot chart coordinates from Basketball Reference, stores them with GORM in PostgreSQL using read/write splitting via HAProxy, and serves them through an NGINX reverse proxy with multi-tier caching, dual-stack IPv4/IPv6 networking, rate limiting, and Prometheus/Grafana observability.

---

## Stack

- **Language & Runtime**: Go (`go 1.23.0`, `toolchain go1.24.2` pinned in `go.mod:3-5`; container runtime uses `golang:1.24-bullseye` builder and `debian:bookworm-slim` runtime in `Dockerfile:2,19`).
- **HTTP Framework**: Go Fiber v2 (`github.com/gofiber/fiber/v2 v2.52.6`).
- **ORM & Database Drivers**: GORM v1.25.12 (`gorm.io/gorm`), PostgreSQL driver v1.6.0 (`gorm.io/driver/postgres`), SQLite driver v1.5.7 (`gorm.io/driver/sqlite` for in-memory testing), and DBResolver v1.5.3 (`gorm.io/plugin/dbresolver`).
- **HTML Scraping**: `github.com/PuerkitoBio/goquery v1.10.3`.
- **Metrics & Observability**: Prometheus client v1.22.0 (`github.com/prometheus/client_golang/prometheus`), Prometheus server (`prom/prometheus:latest`), Grafana (`grafana/grafana:latest`).
- **API Documentation**: Swagger / OpenAPI 2.0 via `github.com/swaggo/swag v1.16.4` and `github.com/swaggo/fiber-swagger v1.3.0`.
- **Reverse Proxy / Cache**: NGINX (`nginx:stable`) with in-memory 30s `api_cache` zone and referer filtering.
- **Testing**: Go standard `testing` with Testify assertions (`github.com/stretchr/testify v1.10.0`).
- **Package Manager**: Go Modules (`go.mod`, `go.sum`).

---

## Layout

- `main.go`: Primary application entrypoint; routes execution between the one-off `import-data` ETL scraper and the live Fiber HTTP server.
- `import.go`: ETL batch orchestration for player totals, advanced stats, game schedules, box scores, and shot charts across season ranges.
- `cmd/`: Utility CLI binaries (e.g., `cmd/generate_key.go` for cryptographic 32-byte API key generation).
- `config/`: Database configuration (`config/database.go`) handling environment variables, HAProxy primary/replica DSN resolution, GORM connection pooling, DB pool metric exporters, and schema migrations.
- `controllers/`: HTTP route handlers and response DTO definitions (`game_controller.go`, `player_total_controller.go`, `player_advanced_controller.go`, `player_shot_chart_controller.go`, `health_controller.go`, `api_key_controller.go`).
- `models/`: GORM database schema structs with database column tags, foreign keys, and indexes (`game.go`, `player_total_stat.go`, `player_advanced_stat.go`, `player_shot_chart.go`, `api_key.go`).
- `routes/`: Fiber route group definitions mapping endpoints to controllers (`game_routes.go`, `player_total_routes.go`, `player_advanced_routes.go`, `player_shot_chart_routes.go`).
- `services/`: Scraper logic and data pipelines extracting HTML tables from Basketball Reference and executing batch upserts (`player_total_scrape_service.go`, `player_advanced_playoffs_scrape_service.go`, `player_shot_chart_scrape_service.go`, `game_scrape_service.go`, `box_score_scrape_service.go`, `line_score_scrape_service.go`, `helpers.go`).
- `utils/`: Reusable components including rate limiting, edge caching headers, structured logging, Prometheus metrics, API key hashing, sleep jitter, and NBA team lookup dictionaries.
- `nginx/`: NGINX reverse proxy configuration (`nginx.conf`) and Dockerfile.
- `prometheus/`: Prometheus configuration (`prometheus.yml`) scraping API replicas and HAProxy.
- `grafana/`: Pre-provisioned dashboards and datasources (`provisioning/`, `dashboards/`).
- `docs/`: OpenAPI schemas (`swagger.json`, `swagger.yaml`), Postman collection, architectural notes, and data migration guides.
- `test/`: Test suites, load testing harness (`loadtest.go`), and load test result logs.
- `.github/`: GitHub Actions workflows (`advance-import.yml`) for automated incremental import window execution and state tracking (`import-state.json`).

---

## Architecture

### Layers & Key Abstractions

1. **Network Ingress & Reverse Proxy**:
   - NGINX (`nginx/nginx.conf:16-63`) listens on container port `8080` (mapped to host `8081`), acts as load balancer across upstream Fiber replicas (`api1:5000`, `api2:5000`, `api3:5000`), caches `/api/` responses in `api_cache` for 30s (`nginx/nginx.conf:33-50`), blocks blacklisted referers (`nginx/nginx.conf:10-14`), and forwards client IP headers (`X-Real-IP`, `X-Forwarded-For`).
2. **HTTP Transport & Middleware**:
   - Dual-stack socket binding on `[::]:5000` via `net.Listen("tcp", ":5000")` passing into Fiber's `app.Listener` (`main.go:140-150`).
   - Global middlewares: CORS (`main.go:102`), Brotli/Gzip compression (`main.go:105-107`), structured JSON logger (`utils/middleware/logger_middleware.go:10`), client rate limiter (`utils/middleware/rate_limiter.go:14`), Prometheus HTTP duration and request counters (`utils/middleware/metrics_middleware.go:11`).
   - Route-level middleware: `EdgeCache` (`utils/middleware/edge_cache.go:32`) sets Cloudflare `Cache-Control` (`immutable`, `semi_dynamic`, `no_cache`) and `Cache-Tag` headers.
3. **Routing & Controllers**:
   - Routes (`routes/*.go`) register endpoints under `/api/*`.
   - Controllers (`controllers/*.go`) parse query filters (e.g. `season`, `team`, `gameId`, `isPlayoff`), handle sorting and pagination, preload GORM associations, and project database entities into sanitized Data Transfer Objects (DTOs) omitting internal database IDs and timestamps.
4. **Data Access & Database Resolver**:
   - GORM ORM (`config/database.go:69-138`) configures `dbresolver`:
     - **Write Ingress**: Port 5437 (`DB_WRITE_PORT`) routes write clauses (`db.Clauses(dbresolver.Write)`) to the PostgreSQL Primary.
     - **Read Ingress**: Port 5438 (`DB_READ_PORT`) routes read queries to PostgreSQL Read Replicas balanced by HAProxy.
     - Connection pool configuration: max open 50, max idle 10, idle timeout 30m, lifetime 2h (`config/database.go:98-102`).
     - Real-time connection pool telemetry gathered every 5s (`utils/metrics/metrics.go:88-125`).
5. **Scraping & ETL Pipeline**:
   - `services/*.go` uses `goquery` to parse Basketball Reference HTML tables, parses column headers via `data-stat` attributes, transforms string values to typed floats/ints (`services/helpers.go:13-31`), and executes batch upserts using `clause.OnConflict` to avoid duplicates (`services/player_total_scrape_service.go:34-110`).

---

## Entrypoints & Flows

### 1. API Server Flow (`main.go`)

1. Process starts and sets up structured JSON logging (`slog`).
2. If `os.Args[1] == "import-data"`, branches into one-off ETL scraper mode (`main.go:53`).
3. Otherwise, initializes Fiber app, attaches global middleware, and connects to PostgreSQL via `config.InitDB(false)`.
4. Registers `/health/live`, `/health/ready`, `/metrics`, `/swagger/*`, `/admin/keys`, and `/api/*` endpoints.
5. Listens on dual-stack port `:5000` with graceful shutdown interception on `SIGINT`/`SIGTERM`.

### 2. Representative Request Flow: `GET /api/games?team=LAL&date=2024-01-15&include=lineScores`

1. **NGINX**: Checks `api_cache` by `$request_uri`. If cache hit, immediately returns 200 with `X-Cache-Status: HIT`. On miss, proxies to `http://go_backend`.
2. **Fiber Pipeline**:
   - `CORS` validates origin.
   - `Compress` inspects `Accept-Encoding`.
   - `StructuredLogger` captures incoming request context.
   - `RateLimiter` inspects `X-Real-IP` (limit 20 req/min per container unless internal IP or `X-Benchmark-Key` provided).
   - `MetricsMiddleware` increments `nba_http_requests_total` and starts latency timer.
   - `EdgeCache(SemiDynamic, "nba-games")` adds headers: `Cache-Control: public, max-age=60, s-maxage=86400, stale-while-revalidate=600` and `Cache-Tag: nba-games`.
3. **Controller (`controllers.GetGames` in `controllers/game_controller.go:176`)**:
   - Resolves team abbreviation `LAL` to `Los Angeles Lakers` via `utils.GetFullName`.
   - Constructs GORM query: filters `WHERE (visitor_team = ? OR home_team = ?) AND date = ?`.
   - Preloads `LineScores` association (`query.Preload("LineScores")`).
   - Executes count query and paginated fetch (`LIMIT 20 OFFSET 0`) routed automatically by DBResolver to HAProxy Read Ingress (:5438).
   - Maps database model `Game` and associated `LineScore` entries into `GameResponseDTO` and `LineScoreDTO`.
   - Responds with `GamesResponse` JSON payload.
4. **Egress**: Metrics timer observes latency in `nba_http_request_duration_seconds`. NGINX writes response to disk cache for 30 seconds.

---

## Data & Integrations

- **Databases**:
  - PostgreSQL 15+ in production, accessed via HAProxy (Write: `DB_WRITE_HOST:5437`, Read: `DB_READ_HOST:5438`, default host `178.105.149.129`).
  - In-memory SQLite (`file::memory:?cache=shared`) for unit/integration testing.
- **Data Models**:
  - `Game`, `LineScore`, `PlayerGameBasicStat`, `PlayerGameAdvStat`, `TeamGameBasicStat`, `TeamGameAdvStat` (`models/game.go`).
  - `PlayerAdvancedStat` (`models/player_advanced_stat.go`): PER, TS%, 3PAr, FTr, WS, VORP, BPM.
  - `PlayerTotalStat` (`models/player_total_stat.go`): Points, rebounds, assists, shooting percentages.
  - `PlayerShotChart` (`models/player_shot_chart.go`): Shot coordinate points (top, left), distance, quarter, shot type.
  - `APIKey` (`models/api_key.go`): SHA-256 key hash, label, revoked flag.
- **External Dependencies**:
  - Scrapes `https://www.basketball-reference.com/` for league totals, playoff stats, schedules, box scores, and shot charts.
- **Authentication & Security Model**:
  - Admin key routes (`/admin/keys`) guarded by `X-Admin-Secret` header (`controllers/api_key_controller.go:14`).
  - API key middleware (`utils/middleware/api_key_middleware.go`) validates `X-API-Key` SHA-256 hash against `api_keys` table using constant-time comparison. _(Note: currently commented out in `main.go:132` for open public access)_.
  - Rate limiter bypass available via `X-Benchmark-Key` header matching `BENCHMARK_KEY` env var (`utils/middleware/rate_limiter.go:35-41`).

---

## Commands

_All commands verified against repository manifests and toolchains:_

- **Download Dependencies**:
  ```bash
  go mod download
  ```
- **Build API Server Binary**:
  ```bash
  go build -v -o nba_go .
  ```
- **Run API Server Locally**:
  ```bash
  go run .
  ```
- **Run ETL Scraper Locally**:
  ```bash
  go run . import-data
  ```
- **Generate API Key**:
  ```bash
  go run cmd/generate_key.go
  ```
- **Execute Test Suite**:
  ```bash
  go test -v ./...
  ```
- **Run Stack with Docker Compose (Local)**:
  ```bash
  make up
  # Maps to: docker-compose -f docker-compose.local.yml up --build -d
  ```
- **Stop Stack with Docker Compose**:
  ```bash
  make down
  # Maps to: docker-compose down
  ```
- **Run Stack (Production Compose)**:
  ```bash
  docker-compose up --build -d
  ```

---

## Conventions

- **Code Style & Packaging**: Standard Go directory layout organized by domain layer (`controllers`, `routes`, `models`, `services`, `utils`).
- **Telemetry & Logging**: Structured logging via Go 1.21+ standard `log/slog` outputting JSON to stdout (`main.go:48-50`).
- **Error Handling**: Custom Fiber error handler translating errors into `{ "error": err.Error() }` with appropriate HTTP status codes (`main.go:92-98`).
- **DTO Separation**: Database models (`models/*`) are strictly separated from HTTP response DTOs (`controllers/*DTO`) to prevent auto-increment IDs, internal GORM timestamps, and deleted records from leaking into JSON responses.
- **Git & Commits**: Active branch is `shooting`. Commit messages combine Conventional Commits (`feat(...)`, `fix(...)`, `docs(...)`) with concise operational tags (`load`, `CDN`, `HA shooting`).

---

## Testing

- **Framework**: Standard Go `testing` package with `stretchr/testify/assert`.
- **Test Locations**:
  - `main_test.go`: Tests HTTP endpoints (`/api/playeradvancedstats/`, `/health/live`, `/health/ready`, `/healthz`, `/readyz`) against an in-memory SQLite database (`setupTestApp`).
  - `test/loadtest_test.go`: Tests load testing logic (page mix weight distribution, URL replacement, cache status parsing).
- **Execution Command**:
  ```bash
  go test -v ./...
  ```
  _(Status: 100% passing across all packages in ~0.8s)_.

---

## Open Questions

- **codebase-memory indexing**: Index for `Volumes-ROG_BLACK-code-review-NBA_Go` is present and matches HEAD (`f7ae315`), but has `auto_watch_off` and `doc_links: error` (predates doc-comment links).
- **API Key Authentication**: API Key authentication middleware is commented out in `main.go:132` (`// app.Use(middleware.APIKeyAuth(db))`) to facilitate open public API access; documentation (`main.go:12-19`) outlines the steps required to re-enable it.
- **Default Database Host Fallback**: `config/database.go:37` and `controllers/health_controller.go:47` default to a hardcoded remote IP (`178.105.149.129`). When developing locally without remote network access, `DB_HOST` / `DB_WRITE_HOST` / `DB_READ_HOST` must be explicitly overridden in `.env`.

---

## Starting Points

- **Adding / Modifying REST Endpoints**: Add GORM model in `models/`, response DTO and handler in `controllers/`, route in `routes/`, and mount in `main.go`.
- **Database Schema & Migrations**: Modify `models/` and update `modelsToMigrate` slice in `config/database.go:115-126`.
- **Scraper Pipeline & Scheduling**: Review `services/` and ETL orchestration in `import.go`, or GitHub Actions scheduler in `.github/workflows/advance-import.yml`.
- **Cache & Rate Limiting Rules**: Inspect `utils/middleware/edge_cache.go` (Cloudflare headers) and `nginx/nginx.conf` (NGINX `api_cache`).
- **Health & Metrics Monitoring**: Update `controllers/health_controller.go` or Prometheus metrics in `utils/metrics/metrics.go` and Grafana dashboards in `grafana/dashboards/`.

---

### Confirmation

- **Scan Root**: `/Volumes/ROG_BLACK/code/review/NBA_Go`
- **Output Status**: Full briefing rendered above in chat. _Writing to `ROOT/.opencode/context.md` was skipped in compliance with Plan Mode rules (which prohibit modifying project files)._
- **Three Things a New Contributor Most Needs to Know**:
  1. **Dual Entrypoints**: `main.go` runs the web API server by default, but switches to the Basketball Reference ETL scraper when invoked with `import-data`.
  2. **Read/Write Database Splitting**: GORM is configured with `dbresolver` to route read queries to PostgreSQL read replicas (HAProxy :5438) and writes/migrations to the primary (HAProxy :5437).
  3. **Strict Model vs. DTO Separation**: Controllers map internal GORM models to dedicated DTO structs; never return raw GORM model instances directly from API handlers.
