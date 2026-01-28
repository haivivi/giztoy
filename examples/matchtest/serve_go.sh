#!/bin/bash
# Serve matchtest results (Go version)

set -e

# Check if running inside bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]] && [[ -z "$TEST_SRCDIR" ]]; then
    echo "ERROR: This script must be run via bazel." >&2
    echo >&2
    echo "Usage:" >&2
    echo "  bazel run //examples/matchtest:serve_go" >&2
    echo >&2
    echo "Note: This target depends on :run_go which requires network/API keys." >&2
    echo "      Run 'bazel build //examples/matchtest:run_go' first if needed." >&2
    exit 1
fi

RUNFILES="${BASH_SOURCE[0]}.runfiles/_main"
[[ -d "$RUNFILES" ]] || RUNFILES="$(dirname "$0")"

BIN="$RUNFILES/$MATCHTEST_BIN"
STATIC="$RUNFILES/$STATIC_DIR"
REPORT="$RUNFILES/$REPORT_FILE"

PORT="${PORT:-8080}"

echo "=== Match Benchmark Server (Go) ==="
echo "Report: $REPORT"
echo "Static: $STATIC"
echo "Port:   $PORT"
echo ""
echo "Open http://localhost:$PORT in browser"
echo ""

exec "$BIN" \
    -load "$REPORT" \
    -serve ":$PORT" \
    -serve-static "$STATIC"
