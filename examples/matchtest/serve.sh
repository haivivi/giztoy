#!/bin/bash
# Serve matchtest results (loads pre-generated report.json)

set -e

RUNFILES="${BASH_SOURCE[0]}.runfiles/_main"
[[ -d "$RUNFILES" ]] || RUNFILES="$(dirname "$0")"

BIN="$RUNFILES/$MATCHTEST_BIN"
STATIC="$RUNFILES/$STATIC_DIR"
REPORT="$RUNFILES/$REPORT_FILE"

# Default port, can override with PORT env
PORT="${PORT:-8080}"

echo "=== Match Benchmark Server ==="
echo "Report: $REPORT"
echo "Static: $STATIC"
echo "Port:   $PORT"
echo ""
echo "Open http://localhost:$PORT in browser"
echo ""

exec "$BIN" \
    --load "$REPORT" \
    --serve ":$PORT" \
    --serve-static "$STATIC"
