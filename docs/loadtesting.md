# Load Testing & Traffic Simulation Guide

The `NBA_Go` repository includes an advanced, high-performance load testing engine written in Go (`test/loadtest.go`). It is designed to stress-test the API across edge caches (Cloudflare), reverse proxies (NGINX), the Go Fiber application server, and PostgreSQL database queries under realistic traffic patterns.

---

## Why Enhanced Load Testing?

In production, APIs often employ multi-tier caching (Cloudflare Edge with `s-maxage` + NGINX in-memory cache). A basic load test querying a fixed URL or a small set of pages will hit the edge cache after the first request, returning sub-15ms responses from CDN memory. While this validates CDN delivery, it completely bypasses the Go origin server, connection pool, and database query engine.

The enhanced load tester solves this by introducing:
1. **Dynamic Query Variation (`-varyParams`, `-complexity`)**: Randomizes combinations of seasons, teams, sort columns, sort order, page sizes, playoff filters, player IDs, and heavy database relation joins.
2. **Cold-Cache Benchmark Mode (`-cacheBust`)**: Injects unique high-resolution nonces to guarantee a 100% cache-miss rate when benchmarking origin and database query execution.
3. **Cache Telemetry Split**: Analyzes `CF-Cache-Status` (`HIT`/`MISS`), `Age`, and `X-Cache` response headers, presenting separate latency profiles for Edge Cache Hits vs. Origin / Database Misses.
4. **Latency Percentiles**: Measures **Min**, **p50 (Median)**, **p75**, **p90**, **p95**, **p99**, **Max**, and **Average** across all requests.
5. **Distributed Client Simulation (`-rotateIPs`)**: Injects rotating `X-Real-IP` and `X-Forwarded-For` headers to simulate thousands of distinct clients.
6. **Benchmark Token Support (`-benchmarkKey`)**: Passes the `X-Benchmark-Key` header to bypass rate limits on origin servers configured with `BENCHMARK_KEY`.
7. **Multi-Endpoint Traffic Generation (`-endpointMix`)**: Distributes traffic across `/api/playeradvancedstats`, `/api/playertotals`, `/api/games`, and `/api/playershotchart`.

---

## CLI Flags Reference

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-url` | `string` | `http://localhost:5000/api/playeradvancedstats` | Target endpoint or base URL to test |
| `-n` | `int` | `100` | Total number of HTTP requests to send |
| `-c` | `int` | `10` | Number of concurrent worker goroutines |
| `-varyParams` | `bool` | `false` | Randomize query parameters (`season`, `team`, `sortBy`, `pageSize`, `isPlayoff`, etc.) |
| `-complexity` | `string` | `standard` | Query complexity level: `standard` or `high` (attaches heavy relation preloads for `/api/games`) |
| `-cacheBust` | `bool` | `false` | Append unique query nonce (`_cb=...`) to force 100% cache misses on edge/proxy layers |
| `-benchmarkKey`| `string` | `$BENCHMARK_KEY` | Token for `X-Benchmark-Key` header to bypass origin rate limiting |
| `-rotateIPs` | `bool` | `false` | Rotate synthetic client IPs (`X-Real-IP` / `X-Forwarded-For`) |
| `-retryOnRateLimit` | `bool` | `false` | Retry HTTP 429 responses with exponential backoff (up to 3 retries) |
| `-endpointMix`| `string` | `""` | Weighted multi-endpoint mix (e.g., `playeradvancedstats:40,playertotals:30,games:30`) |
| `-pageMix` | `string` | `""` | Weighted page mix (e.g., `1-3:60,4-10:30,11-20:10`) |
| `-timeout` | `duration` | `30s` | Per-request HTTP timeout |
| `-key` | `string` | `""` | API key passed in `x-api-key` header |
| `-log` | `string` | `loadtest.log` | Output log file destination |
| `-seed` | `int64` | `-1` | Random seed (`-1` uses current timestamp) |

---

## Usage Recipes

### 1. Realistic Traffic with Parameter Variance (Recommended)
Simulates realistic user traffic with diverse filter permutations across seasons, teams, and sorting:

```bash
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
```

### 2. Pure Origin / Database Stress Test (100% Cold Cache)
Forces all requests to miss edge and reverse-proxy caches to benchmark raw database query execution:

```bash
go run ./test/loadtest.go \
  -url 'https://nba.turbo-data.com/api/playeradvancedstats' \
  -n 1000 \
  -c 15 \
  -varyParams \
  -cacheBust \
  -benchmarkKey 'your-benchmark-token' \
  -log ./test/results.log
```

### 3. Multi-Endpoint Traffic Mix
Exercises multiple domains across the application simultaneously according to traffic weights:

```bash
go run ./test/loadtest.go \
  -url 'https://nba.turbo-data.com' \
  -endpointMix 'playeradvancedstats:40,playertotals:30,games:30' \
  -varyParams \
  -n 2000 \
  -c 20 \
  -benchmarkKey 'your-benchmark-token' \
  -log ./test/results.log
```

### 4. Legacy Execution (Fixed URL with Page Mix)
Runs the traditional page-mix workload:

```bash
go run ./test/loadtest.go \
  -url 'https://nba.turbo-data.com/api/playeradvancedstats?page=1&pageSize=40&sortBy=winShares&ascending=false' \
  -n 500 \
  -c 10 \
  -pageMix '1-3:60,4-10:30,11-20:10' \
  -log ./test/results.log
```

---

## Running Unit Tests

Unit tests validate parameter permutations, cache header parsing, percentiles calculation, and endpoint mixing:

```bash
go test -v ./test/...
```
