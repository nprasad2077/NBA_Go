# NBA_Go

A high-performance NBA statistics REST API built with Go (Fiber), PostgreSQL, and NGINX. Data is scraped from Basketball Reference and served through a load-balanced, containerized stack with dual-stack IPv4/IPv6 support, read/write database splitting, and built-in Prometheus & Grafana observability.

---

## Live Production Endpoints

| Service / Endpoint | URL | Description |
| :--- | :--- | :--- |
| **API Base URL** | `https://nba.turbo-data.com` | Production API root (redirects to Swagger UI) |
| **Swagger UI Docs** | `https://nba.turbo-data.com/swagger/index.html` | Interactive API documentation & sandbox |
| **Player Totals** | `https://nba.turbo-data.com/api/playertotals` | Season totals with pagination & sorting |
| **Games & Box Scores** | `https://nba.turbo-data.com/api/games` | Game schedules, box scores, line scores |
| **Advanced Stats** | `https://nba.turbo-data.com/api/playeradvancedstats` | Advanced metrics (PER, WS, VORP, BPM) |
| **Shot Charts** | `https://nba.turbo-data.com/api/playershotchart` | Shot coordinate & outcome data |
| **Liveness Check** | `https://nba.turbo-data.com/health/live` | Process liveness probe |
| **Readiness Check** | `https://nba.turbo-data.com/health/ready` | Database readiness probe |
| **Metrics** | `https://nba.turbo-data.com/metrics` | Prometheus metrics endpoint |
| **Coolify Dashboard** | `https://turbo-data.com` | Infrastructure & deployment management |

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Traefik (Coolify Ingress Proxy :80 / :443 SSL)              │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│  NGINX (Reverse Proxy, API Cache, Round-Robin Load Balancer) │
│  - Container port 8080 (Mapped to Host :8081)                │
│  - Dual-stack IPv4/IPv6 upstream resolution                  │
│  - 30s response caching for /api/* with stale-while-revalidate│
├──────────────────────────────────────────────────────────────┤
│  API Instances x3 (Fiber on [::]:5000 Dual-Stack)            │
│  ┌──────────┐  ┌─────────────┐  ┌───────────────────────┐    │
│  │  Routes  │→ │ Controllers │→ │ GORM DBResolver (R/W) │    │
│  └──────────┘  └─────────────┘  └───────────┬───────────┘    │
├─────────────────────────────────────────────┼────────────────┤
│  Database Layer (PostgreSQL Cluster)        │                │
│  ┌──────────────────────────────────────────┴─────────────┐  │
│  │ HAProxy Write Ingress (:5437) ➔ Primary DB (:5434)      │  │
│  │ HAProxy Read Ingress (:5438)  ➔ Read Replicas (:5435/6) │  │
│  └────────────────────────────────────────────────────────┘  │
├──────────────────────────────────────────────────────────────┤
│  Observability                                               │
│  - Prometheus (Scrapes /metrics on each instance, Host :9091)│
│  - Grafana (Pre-provisioned dashboards, Host :3001)          │
└──────────────────────────────────────────────────────────────┘
```

### Key Architectural Highlights:
1. **Dual-Stack Networking**: The Go Fiber backend listens on `[::]:5000` (`net.Listen("tcp", ":5000")` with `app.Listener`), accepting incoming connections seamlessly over both IPv6 and IPv4 within Docker and Coolify networks.
2. **Read/Write DB Splitting**: GORM uses the `dbresolver` plugin to automatically route all write operations to the PostgreSQL Primary via HAProxy port 5437, while load balancing read queries across Read Replicas via HAProxy port 5438.
3. **Multi-Layer Caching & Rate Limiting**: NGINX provides an in-memory cache (`api_cache`) for 30 seconds, while Fiber middleware enforces per-client rate limiting (20 req/min per instance, ~60 req/min effective across 3 replicas).

---

### Project Structure

```
.
├── main.go                  # Application entry point (API server or import-data mode)
├── import.go                # Data import orchestration
├── config/
│   └── database.go          # Database connection & DBResolver R/W setup
├── models/                  # GORM models (Game, PlayerAdvancedStat, PlayerTotalStat, etc.)
├── controllers/             # HTTP handlers, DTOs, health checks, pagination
│   ├── game_controller.go
│   ├── health_controller.go # /health/live and /health/ready handlers
│   ├── player_advanced_controller.go
│   ├── player_shot_chart_controller.go
│   └── player_total_controller.go
├── routes/                  # Route registration grouped by domain
├── services/                # Scrapers (Basketball Reference via goquery)
├── utils/
│   ├── middleware/          # Rate limiter, Prometheus metrics, API key auth
│   ├── metrics/             # Prometheus counter & histogram definitions
│   └── security/            # API key hashing & generation
├── nginx/                   # NGINX reverse proxy & cache configuration
├── prometheus/              # Prometheus scrape configuration
├── grafana/                 # Pre-provisioned dashboards & datasources
├── docker-compose.yml       # Production deployment configuration (Coolify)
├── docker-compose.local.yml # Local development configuration (includes local Postgres)
└── docker-compose.override.yml # Override for remote DB development
```

---

## API Endpoints

| Method | Path | Description | Public |
| :--- | :--- | :--- | :---: |
| `GET` | `/` | Redirects to `/swagger/index.html` | Yes |
| `GET` | `/swagger/*` | Interactive Swagger UI documentation | Yes |
| `GET` | `/api/games` | Paginated games with box scores, line scores, team/player stats | Yes |
| `GET` | `/api/playeradvancedstats` | Advanced metrics (PER, WS, VORP, BPM, etc.) | Yes |
| `GET` | `/api/playertotals` | Season totals (points, rebounds, assists, etc.) | Yes |
| `GET` | `/api/playershotchart` | Shot chart coordinates and made/missed outcomes | Yes |
| `GET` | `/health/live` | Process liveness probe | Yes |
| `GET` | `/health/ready` | Database connection readiness probe | Yes |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint | Yes |
| `POST`| `/admin/keys` | Create API key (requires `X-Admin-Secret` header) | Admin |
| `POST`| `/admin/keys/:id/revoke` | Revoke API key (requires `X-Admin-Secret` header) | Admin |

---

### Query Parameters (Data Endpoints)

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `page` | `int` | `1` | Page number |
| `pageSize` | `int` | `20` | Results per page (max 100) |
| `sortBy` | `string` | varies | Field to sort by (e.g., `points`, `date`, `winShares`) |
| `ascending` | `bool` | `false` | Sort ascending (`true`) or descending (`false`) |
| `season` | `int` | - | Filter by season year (e.g., `2024`, `2025`) |
| `team` | `string` | - | Filter by team abbreviation (e.g., `LAL`, `BOS`, `GSW`) |
| `playerId` | `string` | - | Filter by player ID (e.g., `jamesle01`, `curryst01`) |
| `isPlayoff` | `bool` | `false` | Filter for playoff games or stats |

#### Additional Parameters for `/api/games`:
- `date`: Filter by exact date (`YYYY-MM-DD`).
- `gameId`: Filter by specific game ID (e.g., `202501010LAL`).
- `include`: Comma-separated associations to preload (`lineScores`, `playerGameBasicStats`, `playerGameAdvStats`, `teamGameBasicStats`, `teamGameAdvStats`).

---

### Example Requests

#### Production (Live API)
```bash
# Get top scorers for the 2025 season
curl "https://nba.turbo-data.com/api/playertotals?season=2025&sortBy=points&pageSize=10"

# Get a specific game with full box score preloaded
curl "https://nba.turbo-data.com/api/games?gameId=202501010LAL&include=lineScores,playerGameBasicStats,teamGameBasicStats"

# Get LeBron James' advanced stats across career
curl "https://nba.turbo-data.com/api/playeradvancedstats?playerId=jamesle01&sortBy=season&ascending=true"

# Get Stephen Curry's shot chart data for 2024
curl "https://nba.turbo-data.com/api/playershotchart?playerId=curryst01&season=2024"

# Check API readiness
curl "https://nba.turbo-data.com/health/ready"
```

#### Local Development
```bash
# Query local NGINX load balancer (:8081)
curl "http://localhost:8081/api/playertotals?season=2025&pageSize=5"

# Query local Prometheus metrics
curl "http://localhost:8081/metrics"
```

---

## Response Format

All data endpoints return structured JSON with pagination metadata:

```json
{
  "data": [
    {
      "playerId": "curryst01",
      "playerName": "Stephen Curry",
      "season": 2024,
      "team": "GSW",
      "points": 1956,
      "assists": 379,
      "rebounds": 330,
      "threeP": 357,
      "fieldPercent": 0.450
    }
  ],
  "pagination": {
    "total": 450,
    "page": 1,
    "pageSize": 20,
    "pages": 23
  }
}
```

---

## Rate Limiting & Caching

- **Rate Limiting**: Configured per client IP at **20 requests per minute per instance** (~60 req/min across the 3-instance cluster). If exceeded, the API responds with `HTTP 429 Too Many Requests`.
  - **Custom Limits**: Configurable via `RATE_LIMIT_MAX` (default `20`) and `RATE_LIMIT_EXPIRATION_SECONDS` (default `60s`).
  - **Internal Benchmark Bypass**: If `BENCHMARK_KEY` is configured in the environment, passing matching token in `X-Benchmark-Key` header skips rate limiting for automated benchmarks.
- **Multi-Layer Caching**:
  - **Edge CDN (Cloudflare)**: Caches GET requests with `s-maxage=86400` (24h) and `stale-while-revalidate=600`.
  - **NGINX Reverse Proxy**: Provides an in-memory cache (`api_cache`) for **30 seconds** (`keys_zone=api_cache:10m`).
  - Responses include `CF-Cache-Status` (`HIT`/`MISS`) and `X-Cache-Status` (`HIT`/`MISS`) headers.

---

## Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.23+ (for local development)

### Environment Variables
Create a `.env` file in the project root:

```env
DB_HOST=178.105.149.129
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=appdb
DB_PORT=5432
ADMIN_SECRET=your_admin_secret
BENCHMARK_KEY=your_internal_benchmark_token
RATE_LIMIT_MAX=20
```

### Running Locally with Docker

```bash
# Start full local stack (Postgres, 3 API replicas, NGINX, Prometheus, Grafana)
docker compose -f docker-compose.local.yml up --build -d

# Or using Makefile
make up
```

Local service ports:
- **API (via NGINX)**: [http://localhost:8081](http://localhost:8081)
- **Swagger Docs**: [http://localhost:8081/swagger/index.html](http://localhost:8081/swagger/index.html)
- **Prometheus**: [http://localhost:9091](http://localhost:9091)
- **Grafana**: [http://localhost:3001](http://localhost:3001) (`admin` / `testing`)
- **API Direct Instances**: `http://localhost:5001`, `5002`, `5003`

### Initial Data Import

To run the initial schema migrations and scrape Basketball Reference:

```bash
docker compose -f docker-compose.local.yml run --rm db-init
```

This launches the container in one-off `import-data` mode:
1. Executes GORM `AutoMigrate` against the Primary database.
2. Scrapes player advanced stats, regular season totals, playoff totals, and shot charts.
3. Upserts all records into PostgreSQL.

### Stopping Services

```bash
docker compose -f docker-compose.local.yml down
# or
make down
```

---

## Production Deployment (Coolify)

The application is deployed via Coolify on branch `shooting`:
- Uses Traefik as the edge reverse proxy with automated Let's Encrypt SSL.
- Bridges into the `coolify-shared` network to communicate with external databases and proxy services.
- Configured with multi-container services (`api1`, `api2`, `api3`, `nginx`, `prometheus`, `grafana`, `db-init`).

To trigger a redeployment from the command line on the server:
```bash
docker compose -f /data/coolify/applications/<app_uuid>/docker-compose.yaml up -d --build
```

---

## Observability

### Prometheus Metrics
Exposed at `/metrics`:
- `nba_http_requests_total` — Counter partitioned by `method`, `endpoint`, and `status`.
- `nba_http_request_duration_seconds` — Histogram tracking request latencies.
- `nba_db_operations_total` — Counter tracking database read/write queries.

### Grafana Dashboards
Pre-provisioned dashboards in `grafana/dashboards` visualize:
- Request throughput and error rates per endpoint.
- NGINX cache hit ratio.
- Latency percentiles (p50, p95, p99).
- Database read/write distribution across Primary and Replicas.

---

## Swagger API Documentation

To regenerate Swagger documentation after modifying controllers or annotations:

```bash
swag init -g main.go -o docs
```

---

## Testing & Load Testing

### Unit & Integration Tests
```bash
go test -v ./...
```

### Advanced Load Testing (`test/loadtest.go`)
The built-in load testing engine simulates high-volume, realistic traffic patterns across edge caches, reverse proxies, and the origin PostgreSQL database.

See the complete [Load Testing Guide](docs/loadtesting.md) for full details.

#### Key Capabilities:
- **Dynamic Query Randomization (`-varyParams`, `-complexity`)**: Permutes combinations of seasons, teams, sort columns, ascending/descending, page sizes, playoff filters, and heavy database relation joins (`include=lineScores,playerGameBasicStats,teamGameAdvStats`).
- **Cold-Cache Stress Testing (`-cacheBust`)**: Injects unique request nonces to ensure a 100% cache-miss rate for benchmarking raw database execution.
- **Cache Telemetry Split**: Inspects `CF-Cache-Status` (`HIT`/`MISS`) and `Age` headers to provide separate latency profiles for Edge Cache Hits vs. Origin / Database Misses.
- **Latency Percentiles**: Reports **Min**, **p50 (Median)**, **p75**, **p90**, **p95**, **p99**, **Max**, and **Average** latency.
- **Distributed Client Simulation (`-rotateIPs`)**: Injects rotating `X-Real-IP` and `X-Forwarded-For` headers to accurately simulate thousands of distinct clients.
- **Benchmark Token Support (`-benchmarkKey`)**: Passes `X-Benchmark-Key` header to bypass rate limits during testing.
- **Multi-Endpoint Traffic Mix (`-endpointMix`)**: Distributes traffic across `/api/playeradvancedstats`, `/api/playertotals`, `/api/games`, and `/api/playershotchart`.

#### Load Test CLI Flags:

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-url` | `http://localhost:5000/api/playeradvancedstats` | Target endpoint or base URL |
| `-n` | `100` | Number of requests to send |
| `-c` | `10` | Concurrency level (worker goroutines) |
| `-varyParams` | `false` | Enable query parameter & filter randomization |
| `-complexity` | `standard` | Query complexity: `standard` or `high` |
| `-cacheBust` | `false` | Force 100% cold-cache misses on CDN/proxy |
| `-benchmarkKey` | `$BENCHMARK_KEY` | Token for `X-Benchmark-Key` header |
| `-rotateIPs` | `false` | Rotate synthetic client IPs |
| `-endpointMix` | `""` | Multi-endpoint traffic mix (e.g. `playeradvancedstats:40,playertotals:30,games:30`) |
| `-pageMix` | `""` | Weighted page mix (e.g. `1-3:60,4-10:30,11-20:10`) |
| `-retryOnRateLimit` | `false` | Retry 429 responses with backoff |
| `-log` | `loadtest.log` | Output log file destination |

#### Example Usage:

```bash
# 1. Realistic Traffic with Parameter Variance (Recommended)
go run ./test/loadtest.go \
  -url 'https://nba.turbo-data.com/api/playeradvancedstats' \
  -n 3000 \
  -c 20 \
  -varyParams \
  -complexity high \
  -rotateIPs \
  -benchmarkKey 'your-benchmark-token' \
  -pageMix '1-3:60,4-10:30,11-20:10' \
  -log ./test/results.log

# 2. Pure Cold-Cache / Origin Database Benchmark (100% Cache Misses)
go run ./test/loadtest.go \
  -url 'https://nba.turbo-data.com/api/playeradvancedstats' \
  -n 1000 \
  -c 15 \
  -varyParams \
  -cacheBust \
  -benchmarkKey 'your-benchmark-token' \
  -log ./test/results.log

# 3. Multi-Endpoint Traffic Mix
go run ./test/loadtest.go \
  -url 'https://nba.turbo-data.com' \
  -endpointMix 'playeradvancedstats:40,playertotals:30,games:30' \
  -varyParams \
  -n 2000 \
  -c 20 \
  -benchmarkKey 'your-benchmark-token' \
  -log ./test/results.log
```

---

## Tech Stack

| Layer | Technology |
| :--- | :--- |
| **Language** | Go 1.24 |
| **HTTP Framework** | Fiber v2 (`valyala/fasthttp`) |
| **Database** | PostgreSQL 17 (Primary + Read Replicas) |
| **ORM & Routing** | GORM + `dbresolver` (R/W splitting) |
| **Reverse Proxy / Cache** | NGINX Stable (API caching & round-robin) |
| **Edge Routing & SSL** | Traefik v3 (Let's Encrypt automated TLS) |
| **Monitoring** | Prometheus + Grafana |
| **Scraping** | `goquery` (HTML parsing) |
| **Documentation** | Swagger 2.0 (`swaggo/swag`) |
| **Deployment** | Docker, Docker Compose, Coolify |

