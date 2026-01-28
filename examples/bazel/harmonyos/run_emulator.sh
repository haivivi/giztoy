#!/bin/bash
# Run HarmonyOS application on emulator
#
# This script will:
# 1. Check if device/emulator is connected
# 2. Install and launch the application

set -e

# Check if running inside bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
    echo "ERROR: This script must be run via bazel." >&2
    echo >&2
    echo "Usage:" >&2
    echo "  bazel run //examples/bazel/harmonyos:run_emulator -- [hap_file]" >&2
    echo >&2
    echo "Example:" >&2
    echo "  bazel run //examples/bazel/harmonyos:run_emulator" >&2
    echo "  bazel run //examples/bazel/harmonyos:run_emulator -- /path/to/app.hap" >&2
    exit 1
fi

# 获取项目目录 (使用 workspace 路径)
PROJECT_DIR="$BUILD_WORKSPACE_DIRECTORY/examples/bazel/harmonyos/HelloWorld"

echo "=== HarmonyOS Emulator Runner ==="

# DevEco Studio 路径
DEVECO_HOME="${DEVECO_HOME:-/Applications/DevEco-Studio.app/Contents}"
EMULATOR="$DEVECO_HOME/tools/emulator/Emulator"

# Check for DevEco SDK
if [ -z "$DEVECO_SDK_HOME" ]; then
    # Try DevEco Studio bundled SDK first
    if [ -d "$DEVECO_HOME/sdk/default/openharmony" ]; then
        export DEVECO_SDK_HOME="$DEVECO_HOME/sdk/default/openharmony"
    else
        DEVECO_PATHS=(
            "$HOME/Library/Huawei/Sdk"
            "$HOME/Library/DevEco"
        )
        for path in "${DEVECO_PATHS[@]}"; do
            if [ -d "$path" ]; then
                export DEVECO_SDK_HOME="$path"
                break
            fi
        done
    fi
fi

# Find HDC (HarmonyOS Device Connector)
HDC=""
if [ -f "$DEVECO_SDK_HOME/toolchains/hdc" ]; then
    HDC="$DEVECO_SDK_HOME/toolchains/hdc"
elif command -v hdc &> /dev/null; then
    HDC="hdc"
else
    echo "ERROR: hdc (HarmonyOS Device Connector) not found" >&2
    echo "Please ensure DevEco Studio is installed" >&2
    exit 1
fi

echo "HDC: $HDC"

# Check available devices
DEVICES=$("$HDC" list targets 2>/dev/null | tr -d '\r\n' || echo "[Empty]")
echo "Available devices: $DEVICES"

# Check if device is connected
if [ "$DEVICES" = "[Empty]" ] || [ -z "$DEVICES" ]; then
    echo >&2
    echo "ERROR: No device or emulator connected." >&2
    echo >&2
    echo "Please start an emulator first:" >&2
    
    # List available emulator instances
    if [ -f "$EMULATOR" ]; then
        INSTANCES=$("$EMULATOR" -list 2>/dev/null | tr -d '\r' || echo "")
        if [ -n "$INSTANCES" ]; then
            echo >&2
            echo "  Available emulators:" >&2
            echo "$INSTANCES" | while read -r name; do
                echo "    - $name" >&2
            done
            echo >&2
            echo "  Start emulator via command line:" >&2
            FIRST_INSTANCE=$(echo "$INSTANCES" | head -1)
            echo "    $EMULATOR -hvd \"$FIRST_INSTANCE\"" >&2
            echo >&2
            echo "  Or start from DevEco Studio: Tools -> Device Manager" >&2
        fi
    else
        echo "  Open DevEco Studio -> Tools -> Device Manager" >&2
    fi
    exit 1
fi

# Find HAP file
HAP_FILE="${1:-}"
if [ -z "$HAP_FILE" ]; then
    # Try to find in build output
    HAP_FILE=$(find "$PROJECT_DIR" -name "*.hap" 2>/dev/null | head -1)
fi

if [ -z "$HAP_FILE" ] || [ ! -f "$HAP_FILE" ]; then
    echo >&2
    echo "ERROR: HAP file not found" >&2
    echo "Please build the application first:" >&2
    echo "  bazel run //examples/bazel/harmonyos:build_native" >&2
    echo >&2
    echo "Or specify HAP file path:" >&2
    echo "  bazel run //examples/bazel/harmonyos:run_emulator -- /path/to/app.hap" >&2
    exit 1
fi

echo ""
echo "Installing: $HAP_FILE"

# Install HAP on device/emulator
if ! "$HDC" install "$HAP_FILE"; then
    echo "ERROR: Failed to install HAP" >&2
    exit 1
fi

# Get bundle name from HAP (simplified)
BUNDLE_NAME="com.example.hellobazel"

echo ""
echo "Starting application..."
if ! "$HDC" shell aa start -a EntryAbility -b "$BUNDLE_NAME"; then
    echo "ERROR: Failed to start application" >&2
    exit 1
fi

echo ""
echo "✅ Application launched successfully!"
echo "Bundle: $BUNDLE_NAME"
