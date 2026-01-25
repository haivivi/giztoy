#!/bin/bash
# Run benchmark with live web UI (Go version)

set -e

RUNFILES="${BASH_SOURCE[0]}.runfiles/_main"
[[ -d "$RUNFILES" ]] || RUNFILES="$(dirname "$0")"

BIN="$RUNFILES/$MATCHTEST_BIN"
STATIC="$RUNFILES/$STATIC_DIR"
RULES="$RUNFILES/$RULES_DIR"
MODELS="$RUNFILES/$MODELS_DIR"

PORT="${PORT:-8080}"

echo "=== Match Benchmark Live (Go) ==="
echo "Rules:  $RULES"
echo "Models: $MODELS"
echo "Static: $STATIC"
echo "Port:   $PORT"
echo ""

exec "$BIN" \
    -rules "$RULES" \
    -models "$MODELS" \
    -serve ":$PORT" \
    -serve-static "$STATIC" \
    "$@"
