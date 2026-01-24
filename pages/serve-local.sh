#!/bin/bash
# Serve the website locally
# Usage: bazel run //pages:serve-local -- [port]
#    or: ./pages/serve-local.sh [port]  (will auto-invoke bazel)
set -e

# If not running via bazel, re-invoke with bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    WORKSPACE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
    
    echo "🔧 Not running via bazel, invoking: bazel run //pages:serve-local -- $@"
    cd "$WORKSPACE_ROOT"
    exec bazel run //pages:serve-local -- "$@"
fi

PORT="${1:-3000}"
RUNFILES="${BASH_SOURCE[0]}.runfiles"

# Find the built website tarball
WWW_TAR="$RUNFILES/_main/pages/www.tar.gz"
if [[ ! -f "$WWW_TAR" ]]; then
    echo "Error: www.tar.gz not found at $WWW_TAR"
    exit 1
fi

# Extract to temp directory
SITE_DIR=$(mktemp -d)
trap "rm -rf $SITE_DIR" EXIT

tar -xzf "$WWW_TAR" -C "$SITE_DIR"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🌌 Giztoy Pages"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  🏠 Homepage:      http://localhost:$PORT"
echo "  📚 Documentation: http://localhost:$PORT/docs/"
echo ""
echo "  Press Ctrl+C to stop"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd "$SITE_DIR"
python3 -m http.server "$PORT"
