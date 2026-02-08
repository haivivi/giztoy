#!/bin/bash
# Run benchmark with live web UI (no caching)

set -e

# Check if running inside bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]] && [[ -z "$TEST_SRCDIR" ]]; then
    echo "ERROR: This script must be run via bazel." >&2
    echo >&2
    echo "Usage:" >&2
    echo "  bazel run //examples/matchtest:live -- [args...]" >&2
    echo >&2
    echo "Example:" >&2
    echo "  bazel run //examples/matchtest:live -- --model qwen/turbo" >&2
    exit 1
fi

RUNFILES="${BASH_SOURCE[0]}.runfiles/_main"
[[ -d "$RUNFILES" ]] || RUNFILES="$(dirname "$0")"

BIN="$RUNFILES/$MATCHTEST_BIN"
STATIC="$RUNFILES/$STATIC_DIR"
RULES="$RUNFILES/$RULES_DIR"
MODELS="$RUNFILES/$MODELS_DIR"

PORT="${PORT:-8080}"

echo "=== Match Benchmark (Live) ==="
echo "Rules:  $RULES"
echo "Models: $MODELS"
echo "Static: $STATIC"
echo "Port:   $PORT"
echo ""

exec "$BIN" \
    --rules "$RULES" \
    --models "$MODELS" \
    --serve ":$PORT" \
    --serve-static "$STATIC" \
    "$@"
