# NBA_Go

A high-performance NBA statistics REST API built with Go (Fiber), PostgreSQL, and NGINX. Data is scraped from Basketball Reference and served through a load-balanced, containerized stack with built-in observability.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  NGINX (reverse proxy / round-robin load balancer :8080) │
├──────────────────────────────────────────────────────────┤
│  API Instance x3 (Fiber :5000 each)                      │
│  ┌──────────┐  ┌─────────────┐  ┌───────────────────┐    │
│  │  Routes  │→ │ Controllers │→ │ Services (scraper) │   │
│  └──────────┘  └─────────────┘  └───────────────────┘   │
├──────────────────────────────────────────────────────────┤
│  PostgreSQL 15 (GORM ORM)                                │
├──────────────────────────────────────────────────────────┤
│  Prometheus + Grafana (metrics & dashboards)             │
└──────────────────────────────────────────────────────────┘
```

### Project Structure

```
.
├── main.go                  # Entry point (API server or import-data mode)
├── import.go                # Bulk data import orchestration
├── config/                  # Database initialization
├── models/                  # GORM models (Game, PlayerAdvancedStat, PlayerTotalStat, etc.)
├── controllers/             # HTTP handlers, DTOs, pagination, filtering, sorting
├── routes/                  # Route registration grouped by domain
├── services/                # Web scrapers (Basketball Reference via goquery)
├── utils/
│   ├── middleware/          # Rate limiter, metrics, API key auth
│   ├── metrics/             # Prometheus counter/histogram definitions
│   └── security/            # API key generation & hashing
├── nginx/                   # NGINX load balancer config
├── prometheus/              # Prometheus scrape config
├── grafana/                 # Pre-provisioned dashboards & datasources
├── docker-compose.yml       # Production (Coolify)
├── docker-compose.local.yml # Local development (includes Postgres)
└── docker-compose.override.yml # Override for remote DB development
```

## API Endpoints

| Method | Path                       | Description                                               |
| ------ | -------------------------- | --------------------------------------------------------- |
| GET    | `/api/games`               | Game data with box scores, line scores, team/player stats |
| GET    | `/api/playeradvancedstats` | Advanced stats (PER, WS, VORP, BPM, etc.)                 |
| GET    | `/api/playertotals`        | Season totals (points, rebounds, assists, etc.)           |
| GET    | `/api/playershotchart`     | Shot chart coordinate data                                |
| GET    | `/swagger/*`               | Interactive Swagger UI documentation                      |
| GET    | `/metrics`                 | Prometheus metrics endpoint                               |
| POST   | `/admin/keys`              | Create API key (requires `X-Admin-Secret` header)         |

### Query Parameters (all data endpoints)

| Parameter   | Type   | Description                                  |
| ----------- | ------ | -------------------------------------------- |
| `page`      | int    | Page number (default: 1)                     |
| `pageSize`  | int    | Results per page (default: 20)               |
| `sortBy`    | string | Field to sort by (varies per endpoint)       |
| `ascending` | bool   | Sort direction (default: false / descending) |
| `season`    | int    | Filter by season year (e.g., 2025)           |
| `team`      | string | Filter by team abbreviation (e.g., LAL, BOS) |
| `playerId`  | string | Filter by player ID (e.g., jamesle01)        |
| `isPlayoff` | bool   | Filter for playoff stats                     |

#### Games-specific parameters

| Parameter | Type   | Description                                                                                                                                   |
| --------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `date`    | string | Filter by date (YYYY-MM-DD)                                                                                                                   |
| `gameId`  | string | Filter by specific game ID                                                                                                                    |
| `include` | string | Comma-separated associations to preload: `lineScores`, `playerGameBasicStats`, `playerGameAdvStats`, `teamGameBasicStats`, `teamGameAdvStats` |

### Example Requests

```bash
# Get top scorers for the 2025 season
curl "http://localhost:8080/api/playertotals?season=2025&sortBy=points&pageSize=10"

# Get a specific game with full box score
curl "http://localhost:8080/api/games?gameId=202501010LAL&include=lineScores,playerGameBasicStats,teamGameBasicStats"

# Get LeBron's advanced stats across all seasons
curl "http://localhost:8080/api/playeradvancedstats?playerId=jamesle01&sortBy=season&ascending=true"

# Get shot chart data for Curry in 2024
curl "http://localhost:8080/api/playershotchart?playerId=curryst01&season=2024"
```

### Response Format

All endpoints return paginated JSON:

```json
{
  "data": [...],
  "pagination": {
    "total": 450,
    "page": 1,
    "pageSize": 20,
    "pages": 22
  }
}
```

## Rate Limiting

The API enforces a per-IP rate limit of **20 requests per minute per instance**. With 3 instances behind NGINX round-robin, the effective limit is ~60 requests/minute per client.

Exceeding the limit returns:

```json
HTTP 429
{"error": "Rate limit exceeded. Try again later."}
```

## Getting Started

### Prerequisites

- Docker & Docker Compose
- Go 1.23+ (for local development)
- A `.env` file with database credentials

### Environment Variables

```env
DB_HOST=postgres
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_db
DB_PORT=5432
ADMIN_SECRET=your_admin_secret
```

### Local Development

```bash
# Start everything (Postgres, 3 API instances, NGINX, Prometheus, Grafana)
docker-compose -f docker-compose.local.yml up --build -d

# Or use the Makefile shortcut
make up
```

Services will be available at:

| Service                 | URL                                     |
| ----------------------- | --------------------------------------- |
| API (via NGINX)         | <http://localhost:8081>                 |
| Prometheus              | <http://localhost:9090>                 |
| Grafana                 | <http://localhost:3001> (admin/testing) |
| API instance 1 (direct) | <http://localhost:5001>                 |
| API instance 2 (direct) | <http://localhost:5002>                 |
| API instance 3 (direct) | <http://localhost:5003>                 |

### Importing Data

The application provides a dual-mode entry point. To run data migrations and imports:

#### 1. Local CLI Execution (Recommended for automated & targeted imports)
```bash
# Export environment variables from .env (e.g. remote or local database)
export $(grep -v '^#' .env | xargs)

# Run the import pipeline
go run . import-data
```

#### 2. Local Docker Development Stack
```bash
docker-compose -f docker-compose.local.yml run --rm db-init
```

#### 3. Production / Coolify Deployment
```bash
docker compose --profile init run --rm db-init
```

---

### Ingestion Pipeline & Scraping Architecture

The data import engine (`import.go` & `services/`) features a resilient, multi-stage ingestion workflow designed to safely scrape and persist NBA statistics without rate limits or data loss:

#### 1. Auto-Detect Missing Box Scores Engine (`importMissingBoxScores`)
- **Dynamic Database Discovery**: Automatically queries PostgreSQL for any games in the `games` table that lack corresponding records in `line_scores` (`WHERE game_id NOT IN (SELECT DISTINCT game_id FROM line_scores WHERE deleted_at IS NULL)`).
- **Zero Hardcoding**: Eliminates the need to manually configure date ranges or game IDs when fixing missing data across multiple historical seasons (e.g., 2008, 2013, 2017).
- **20-Game Batches**: Groups detected missing games into safe 20-game chunks with 2 concurrent workers and staggered worker starts.
- **Immediate Incremental Persistence**: Scraped data (`line_scores`, `player_game_basic_stats`, `player_game_adv_stats`, `team_game_basic_stats`, `team_game_adv_stats`) is immediately committed and upserted into PostgreSQL after every batch.
- **Inter-Batch Cool-Off**: Enforces a 20-second pause ($\pm 25\%$ jitter) between batches to maintain compliant request rates against upstream sources.
- **Defensive URL Construction**: Automatically constructs `/boxscores/{gameID}.html` if a game record has an empty `box_score_url`.

#### 2. Date-Range Chunked Ingestion (`importBoxScores`)
- **20-Day Temporal Session Chunks**: Large date spans (such as an entire 9-month season) are partitioned into 20-day sliding windows (~80–120 games per chunk).
- **Off-Season Smart Skipping**: Summer months (July–October) with 0 games are automatically identified and skipped in milliseconds without triggering scraping pauses.

#### 3. Fault Tolerance & Safety Guarantees
- **Graceful Interrupt Handling (`Ctrl+C`)**: Captures `SIGINT` and `SIGTERM` via `context.Context`. If interrupted, all data from completed batches is safely preserved in PostgreSQL.
- **Idempotent Resumption**: Re-running `go run . import-data` automatically discovers only the remaining pending games, skipping all previously completed games.

---

### Database Verification Queries

Run the following SQL queries in PostgreSQL to verify data completeness and monitor ingestion progress:

```sql
-- 1. Check count of remaining missing games (returns 0 when fully complete)
SELECT count(*) AS remaining_missing_games
FROM games g
LEFT JOIN line_scores ls ON g.game_id = ls.game_id AND ls.deleted_at IS NULL
WHERE ls.game_id IS NULL AND g.deleted_at IS NULL;

-- 2. Inspect recently imported line scores
SELECT ls.game_id, g.date, ls.team, ls.q1, ls.q2, ls.q3, ls.q4, ls.ot1, ls.total
FROM line_scores ls
JOIN games g ON ls.game_id = g.game_id
ORDER BY ls.updated_at DESC
LIMIT 20;

-- 3. Verify total games vs total line scores
SELECT 
    (SELECT count(*) FROM games WHERE deleted_at IS NULL) AS total_games,
    (SELECT count(DISTINCT game_id) FROM line_scores WHERE deleted_at IS NULL) AS games_with_boxscores,
    (SELECT count(*) FROM line_scores WHERE deleted_at IS NULL) AS total_line_scores;
```

---

### Stopping

```bash
docker compose down
# or
make down
```

## Production Deployment

The main `docker-compose.yml` is configured for deployment on Coolify with an external `coolify` network. It expects the database to be provisioned separately (no local Postgres service).

The `docker-compose.override.yml` disables the local Postgres container and removes `depends_on` constraints, allowing API services to connect to a remote database specified in `.env`.

## Observability

### Prometheus Metrics

Exposed at `/metrics` on each API instance. Tracked metrics:

- `nba_http_requests_total` — counter by method, endpoint, status
- `nba_http_request_duration_seconds` — histogram by method, endpoint
- `nba_db_operations_total` — counter by operation, entity

### Grafana

Pre-provisioned dashboards visualize request rates and endpoint usage. Access at port 3001 (local) or 3000 (production).

## API Key Management (Optional)

API key authentication is available but currently disabled. To create keys for future use:

```bash
# Create a key
curl -XPOST http://localhost:8080/admin/keys \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -d '{"label":"my-app"}'
# → {"id":1, "apiKey":"ab12cd…"}

# Revoke a key
curl -XPOST http://localhost:8080/admin/keys/1/revoke \
  -H "X-Admin-Secret: $ADMIN_SECRET"
```

To enforce API keys, uncomment `app.Use(middleware.APIKeyAuth(db))` in `main.go`.

## Regenerating Swagger Docs

```bash
swag init -g main.go -o docs
```

## Running Tests

```bash
go test -v .
```

### Load Testing

```bash
cd test
go run loadtest.go -n 100 -c 10 -url "http://localhost:8080/api/playeradvancedstats?page=1&pageSize=20" -log results.log
```

## Tech Stack

| Component        | Technology              |
| ---------------- | ----------------------- |
| Language         | Go 1.23+                |
| Framework        | Fiber v2                |
| ORM              | GORM                    |
| Database         | PostgreSQL 15           |
| Scraping         | goquery                 |
| Load Balancer    | NGINX                   |
| Monitoring       | Prometheus + Grafana    |
| Docs             | Swagger (swaggo)        |
| Containerization | Docker + Docker Compose |

## Workflows

### 1. Ingesting a New Season from Scratch
1. Set the target season in [`import.go`](file:///Volumes/ROG_BLACK/code/update/NBA_Go/import.go) for `importPlayerTotalsScrape`, `importPlayerAdvanced`, `importGameSchedules`, and `importMarkPlayoffGames`.
2. Enable schedule and season totals imports in [`main.go`](file:///Volumes/ROG_BLACK/code/update/NBA_Go/main.go).
3. Run `go run . import-data` to ingest schedules and season totals.
4. Run `importMissingBoxScores` to automatically ingest all game box scores and line scores in 20-game chunks.

### 2. Auto-Detecting & Filling Data Gaps
1. Ensure `importMissingBoxScores(db)` is active in [`main.go`](file:///Volumes/ROG_BLACK/code/update/NBA_Go/main.go).
2. Run `go run . import-data`.
3. The engine automatically finds any missing games in PostgreSQL across all seasons, splits them into 20-game batches, and ingests them with immediate DB commits.

### 3. Local Development & API Testing
1. Start the local stack with `make up` or `docker-compose -f docker-compose.local.yml up -d`.
2. Open Swagger documentation at `http://localhost:8081/swagger/index.html`.
3. View Grafana metrics dashboards at `http://localhost:3001` (login: `admin` / `testing`).

