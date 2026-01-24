#!/bin/bash
# Run HarmonyOS application on emulator
#
# Requirements:
# - DevEco Studio installed with emulator
# - Application already built (.hap file)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== HarmonyOS Emulator Runner ==="

# Check for DevEco SDK
if [ -z "$DEVECO_SDK_HOME" ]; then
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

# Find HDC (HarmonyOS Device Connector)
HDC=""
if [ -f "$DEVECO_SDK_HOME/toolchains/hdc" ]; then
    HDC="$DEVECO_SDK_HOME/toolchains/hdc"
elif command -v hdc &> /dev/null; then
    HDC="hdc"
else
    echo "ERROR: hdc (HarmonyOS Device Connector) not found"
    echo "Please ensure DevEco Studio is installed"
    exit 1
fi

echo "Using HDC: $HDC"

# List available devices
echo ""
echo "Available devices:"
"$HDC" list targets

# Find HAP file
HAP_FILE="${1:-}"
if [ -z "$HAP_FILE" ]; then
    # Try to find in build output
    HAP_FILE=$(find "$SCRIPT_DIR/HelloWorld" -name "*.hap" 2>/dev/null | head -1)
fi

if [ -z "$HAP_FILE" ] || [ ! -f "$HAP_FILE" ]; then
    echo ""
    echo "ERROR: HAP file not found"
    echo "Please build the application first:"
    echo "  bazel run //examples/bazel/harmonyos:build_native"
    echo ""
    echo "Or specify HAP file path:"
    echo "  $0 /path/to/app.hap"
    exit 1
fi

echo ""
echo "Installing: $HAP_FILE"

# Install HAP on device/emulator
"$HDC" install "$HAP_FILE"

# Get bundle name from HAP (simplified)
BUNDLE_NAME="com.example.hellobazel"

echo ""
echo "Starting application..."
"$HDC" shell aa start -a EntryAbility -b "$BUNDLE_NAME"

echo ""
echo "Application launched successfully!"
echo "Bundle: $BUNDLE_NAME"
