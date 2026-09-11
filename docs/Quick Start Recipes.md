# Quick Start Recipes

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