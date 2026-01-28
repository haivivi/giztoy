#!/bin/bash
# Run benchmark with live web UI (Go version)

set -e

# Check if running inside bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]] && [[ -z "$TEST_SRCDIR" ]]; then
    echo "ERROR: This script must be run via bazel."
    echo ""
    echo "Usage:"
    echo "  bazel run //examples/matchtest:live_go -- [args...]"
    echo ""
    echo "Example:"
    echo "  bazel run //examples/matchtest:live_go -- -model qwen/turbo"
    exit 1
fi

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
