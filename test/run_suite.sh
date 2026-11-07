#!/usr/bin/env bash
# ==============================================================================
# NBA_Go Automated Load Testing Suite
#
# Usage:
#   ./test/run_suite.sh [scenario] [options]
#
# Scenarios:
#   all         - Run all scenarios in sequence (default)
#   warm        - 1. Warm Cache (Pure Edge Delivery / high HIT rate)
#   cold        - 2. Pure Origin / Database Stress (100% Cache Misses)
#   mix         - 3. Multi-Endpoint Traffic Mix
#   variance    - 4. Realistic Parameter Variance (High Complexity)
#   legacy      - 5. Legacy Fixed-URL Page Mix
#
# Environment Variables:
#   API_HOST        - Target base host (default: https://nba.space-city.dev)
#   BENCHMARK_KEY   - Bypass token for X-Benchmark-Key header
#   LOADTEST_BIN    - Path to compiled loadtest binary (default: test/bin/loadtest)
#   QUICK           - If set to 1, scales request counts down for quick smoke tests
#   PAUSE_SECONDS   - Seconds to pause between sequential runs (default: 4)
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
HOST="${API_HOST:-https://nba.space-city.dev}"
KEY="${BENCHMARK_KEY:-}"
BIN="${LOADTEST_BIN:-$SCRIPT_DIR/bin/loadtest}"
PAUSE="${PAUSE_SECONDS:-4}"
IS_QUICK="${QUICK:-0}"

# Parse optional arguments
TARGET_SCENARIO="all"
for arg in "$@"; do
  case "$arg" in
    --quick|-q)
      IS_QUICK=1
      ;;
    --help|-h)
      echo "NBA_Go Load Testing Suite"
      echo ""
      echo "Usage: $0 [scenario] [--quick]"
      echo ""
      echo "Scenarios:"
      echo "  all         Run all test scenarios (default)"
      echo "  warm        Pure Edge / Warm Cache test"
      echo "  cold        Pure Origin / Database Stress (cache-bust)"
      echo "  mix         Multi-endpoint traffic mix"
      echo "  variance    Realistic high-complexity parameter variance"
      echo "  legacy      Legacy fixed-URL page mix"
      echo ""
      echo "Options:"
      echo "  --quick, -q Run with reduced request counts (smoke test)"
      echo ""
      echo "Environment variables:"
      echo "  API_HOST       Target base URL (default: https://nba.space-city.dev)"
      echo "  BENCHMARK_KEY  X-Benchmark-Key token (bypasses rate limiter)"
      exit 0
      ;;
    warm|cold|mix|variance|legacy|all)
      TARGET_SCENARIO="$arg"
      ;;
    *)
      echo "Unknown argument: $arg"
      echo "Run with --help for usage."
      exit 1
      ;;
  esac
done

# Ensure binary is compiled
if [ ! -f "$BIN" ] || [ "$SCRIPT_DIR/loadtest.go" -nt "$BIN" ]; then
  if command -v go >/dev/null 2>&1; then
    echo "🔨 Building loadtest binary ($BIN)..."
    mkdir -p "$(dirname "$BIN")"
    (cd "$REPO_ROOT" && go build -o "$BIN" "$SCRIPT_DIR/loadtest.go")
  else
    if [ ! -f "$BIN" ]; then
      echo "❌ Error: loadtest binary not found at $BIN and 'go' is not installed."
      echo "Please install Go or set LOADTEST_BIN to a precompiled binary."
      exit 1
    fi
  fi
fi

# Set up timestamped output directory
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="$SCRIPT_DIR/results/$TIMESTAMP"
mkdir -p "$OUT_DIR"

echo "========================================================="
echo "🏀 NBA_Go Load Test Suite Runner"
echo "========================================================="
echo "Target Host : $HOST"
echo "Output Dir  : $OUT_DIR"
echo "Binary      : $BIN"
if [ -n "$KEY" ]; then
  echo "Auth Token  : Set (X-Benchmark-Key enabled)"
else
  echo "Auth Token  : None (relying on server-side IP/env rules)"
fi
if [ "$IS_QUICK" -eq 1 ]; then
  echo "Mode        : ⚡ QUICK (smoke test counts)"
fi
echo "Scenario    : $TARGET_SCENARIO"
echo "========================================================="
echo ""

# Helper to execute an individual test
run_test() {
  local name="$1"
  local desc="$2"
  shift 2

  echo "---------------------------------------------------------"
  echo "▶ [$name] $desc"
  echo "---------------------------------------------------------"

  local log_file="$OUT_DIR/${name}.log"
  local -a cmd=("$BIN" -log "$log_file")

  if [ -n "$KEY" ]; then
    cmd+=(-benchmarkKey "$KEY")
  fi

  # Append scenario specific flags
  cmd+=("$@")

  "${cmd[@]}"

  echo "📄 Output logged to: $log_file"
  echo ""

  if [ "$PAUSE" -gt 0 ]; then
    sleep "$PAUSE"
  fi
}

# ------------------------------------------------------------------------------
# Scenarios
# ------------------------------------------------------------------------------

# 1. Warm Cache (Pure Edge CDN Delivery)
run_scenario_warm() {
  local n=1000
  local c=20
  if [ "$IS_QUICK" -eq 1 ]; then n=50; c=5; fi

  run_test "1_warm_cache" "Edge Cache Delivery Benchmark (High HIT rate)" \
    -url "$HOST/api/playeradvancedstats?page=1&pageSize=40" \
    -n "$n" \
    -c "$c" \
    -pageMix '1:100'
}

# 2. Pure Origin / Database Stress Test (100% Cache Misses)
run_scenario_cold() {
  local n=1000
  local c=15
  if [ "$IS_QUICK" -eq 1 ]; then n=30; c=5; fi

  run_test "2_cold_origin" "Pure Database Stress Test (100% Cold Cache / Cache-Busting)" \
    -url "$HOST/api/playeradvancedstats" \
    -n "$n" \
    -c "$c" \
    -varyParams \
    -cacheBust
}

# 3. Multi-Endpoint Traffic Mix
run_scenario_mix() {
  local n=2000
  local c=20
  if [ "$IS_QUICK" -eq 1 ]; then n=60; c=6; fi

  run_test "3_endpoint_mix" "Multi-Endpoint Traffic Mix across Core Endpoints" \
    -url "$HOST" \
    -endpointMix 'playeradvancedstats:40,playertotals:30,games:30' \
    -varyParams \
    -n "$n" \
    -c "$c"
}

# 4. Realistic Parameter Variance (High Complexity)
run_scenario_variance() {
  local n=3000
  local c=20
  if [ "$IS_QUICK" -eq 1 ]; then n=60; c=6; fi

  run_test "4_realistic_variance" "Realistic Traffic with Parameter Variance & Deep Filters" \
    -url "$HOST/api/playeradvancedstats" \
    -n "$n" \
    -c "$c" \
    -varyParams \
    -complexity high \
    -rotateIPs \
    -pageMix '1-3:60,4-10:30,11-20:10'
}

# 5. Legacy Fixed-URL Page Mix
run_scenario_legacy() {
  local n=500
  local c=10
  if [ "$IS_QUICK" -eq 1 ]; then n=30; c=5; fi

  run_test "5_legacy_page_mix" "Legacy Execution (Fixed URL with Page Mix)" \
    -url "$HOST/api/playeradvancedstats?page=1&pageSize=40&sortBy=winShares&ascending=false" \
    -n "$n" \
    -c "$c" \
    -pageMix '1-3:60,4-10:30,11-20:10'
}

# ------------------------------------------------------------------------------
# Dispatcher
# ------------------------------------------------------------------------------

case "$TARGET_SCENARIO" in
  warm)
    run_scenario_warm
    ;;
  cold)
    run_scenario_cold
    ;;
  mix)
    run_scenario_mix
    ;;
  variance)
    run_scenario_variance
    ;;
  legacy)
    run_scenario_legacy
    ;;
  all)
    run_scenario_warm
    run_scenario_cold
    run_scenario_mix
    run_scenario_variance
    run_scenario_legacy
    ;;
esac

echo "========================================================="
echo "🎉 Load test execution complete!"
echo "📁 All scenario logs saved in: $OUT_DIR"
echo "========================================================="
