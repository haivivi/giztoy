#!/bin/bash
# ESP-IDF Build Helper Script
# This script sets up the environment and builds ESP-IDF projects with Zig support.
#
# Usage:
#   ./build.sh <project_dir> [command]
#
# Commands:
#   build     - Build the project (default)
#   flash     - Flash to device
#   monitor   - Monitor serial output
#   clean     - Clean build artifacts
#
# Examples:
#   ./build.sh led_strip_flash build
#   ./build.sh led_strip_flash flash
#   ./build.sh led_strip_flash monitor

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check arguments
if [ $# -lt 1 ]; then
    echo "Usage: $0 <project_dir> [command]"
    echo "Commands: build, flash, monitor, clean"
    exit 1
fi

PROJECT_DIR="$1"
COMMAND="${2:-build}"
PORT="${ESP_PORT:-/dev/cu.usbmodem1301}"

# Full project path
if [[ "$PROJECT_DIR" == /* ]]; then
    PROJECT_PATH="$PROJECT_DIR"
else
    PROJECT_PATH="$SCRIPT_DIR/$PROJECT_DIR"
fi

if [ ! -d "$PROJECT_PATH" ]; then
    log_error "Project directory not found: $PROJECT_PATH"
    exit 1
fi

# =============================================================================
# Step 1: Ensure Zig is available via Bazel
# =============================================================================
log_info "Checking Zig installation via Bazel..."

cd "$PROJECT_ROOT"

# Build Zig toolchain if not exists
BAZEL_ZIG_PATH=$(bazel cquery "@embed_zig_toolchain//:zig_files" --output=files 2>/dev/null | head -1 | xargs dirname 2>/dev/null || echo "")

if [ -z "$BAZEL_ZIG_PATH" ] || [ ! -f "$BAZEL_ZIG_PATH/zig" ]; then
    log_info "Downloading Zig via Bazel..."
    bazel build @embed_zig_toolchain//:zig_files
    
    # Get the path again
    BAZEL_EXTERNAL=$(bazel info output_base 2>/dev/null)/external/+embed_zig+embed_zig_toolchain
    if [ -f "$BAZEL_EXTERNAL/zig" ]; then
        BAZEL_ZIG_PATH="$BAZEL_EXTERNAL"
    fi
fi

if [ ! -f "$BAZEL_ZIG_PATH/zig" ]; then
    # Fallback to manual detection
    BAZEL_EXTERNAL=$(find /private/var/tmp/_bazel_* -name "+embed_zig+embed_zig_toolchain" -type d 2>/dev/null | head -1)
    if [ -f "$BAZEL_EXTERNAL/zig" ]; then
        BAZEL_ZIG_PATH="$BAZEL_EXTERNAL"
    fi
fi

if [ ! -f "$BAZEL_ZIG_PATH/zig" ]; then
    log_error "Could not find Zig compiler. Please run: bazel build @embed_zig_toolchain//:zig_files"
    exit 1
fi

export ZIG_INSTALL="$BAZEL_ZIG_PATH"
log_info "Using Zig: $ZIG_INSTALL/zig"
$ZIG_INSTALL/zig version

# =============================================================================
# Step 2: Setup ESP lib symlink
# =============================================================================
log_info "Setting up ESP lib symlink..."

BAZEL_ESP_LIB=$(find /private/var/tmp/_bazel_* -path "*+embed_zig+embed_zig/lib/esp" -type d 2>/dev/null | head -1)
if [ -d "$BAZEL_ESP_LIB" ]; then
    mkdir -p "$SCRIPT_DIR/lib"
    if [ ! -L "$SCRIPT_DIR/lib/esp" ]; then
        ln -sf "$BAZEL_ESP_LIB" "$SCRIPT_DIR/lib/esp"
        log_info "Created symlink: lib/esp -> $BAZEL_ESP_LIB"
    fi
fi

# =============================================================================
# Step 3: Setup ESP-IDF environment
# =============================================================================
log_info "Setting up ESP-IDF environment..."

IDF_PATH="${IDF_PATH:-/Users/idy/esp/esp-adf/esp-idf}"
ADF_PATH="${ADF_PATH:-/Users/idy/esp/esp-adf}"

if [ ! -f "$IDF_PATH/export.sh" ]; then
    log_error "ESP-IDF not found at: $IDF_PATH"
    log_error "Please set IDF_PATH environment variable"
    exit 1
fi

source "$IDF_PATH/export.sh" > /dev/null 2>&1 || true
log_info "ESP-IDF: $IDF_PATH"

# =============================================================================
# Step 4: Execute command
# =============================================================================
cd "$PROJECT_PATH"

case "$COMMAND" in
    build)
        log_info "Building project: $PROJECT_DIR"
        idf.py build
        ;;
    flash)
        log_info "Flashing to $PORT..."
        idf.py -p "$PORT" flash
        ;;
    monitor)
        log_info "Monitoring $PORT..."
        idf.py -p "$PORT" monitor
        ;;
    clean)
        log_info "Cleaning build artifacts..."
        rm -rf build .zig-cache sdkconfig sdkconfig.old managed_components dependencies.lock
        ;;
    *)
        log_error "Unknown command: $COMMAND"
        echo "Available commands: build, flash, monitor, clean"
        exit 1
        ;;
esac

log_info "Done!"
