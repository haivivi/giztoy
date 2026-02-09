#!/bin/bash
# Run a Luau test script with the Go runner
# Can be called directly or via bazel test

set -e

# Check if running inside bazel (TEST_SRCDIR is set by bazel test)
if [[ -z "$TEST_SRCDIR" ]]; then
    # Not in bazel environment, invoke bazel test
    # Determine which test to run based on script name or arguments
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    WORKSPACE_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
    
    # Default to running all tests, or specific test if provided
    TARGET="${1:-//luau/tests:all}"
    
    echo "Not in bazel environment, invoking: bazel test $TARGET"
    cd "$WORKSPACE_ROOT"
    exec bazel test "$TARGET" --test_output=all "${@:2}"
fi

# Running inside bazel sandbox
RUNNER="$1"
SCRIPT="$2"
LIBS_DIR="$3"
RUNTIME="${4:-minimal}"  # Default to minimal runtime

# In bazel sandbox, files are in runfiles directory
# Try to find the libs directory
if [[ -d "$LIBS_DIR" ]] && [[ -d "$LIBS_DIR/haivivi" ]]; then
    : # LIBS_DIR is already correct
elif [[ -d "luau/libs/haivivi" ]]; then
    LIBS_DIR="luau/libs"
elif [[ -d "../luau/libs/haivivi" ]]; then
    LIBS_DIR="../luau/libs"
else
    # Try to find via runfiles
    SCRIPT_DIR=$(dirname "$SCRIPT")
    if [[ -d "$SCRIPT_DIR/../libs/haivivi" ]]; then
        LIBS_DIR="$SCRIPT_DIR/../libs"
    else
        echo "ERROR: Cannot find libs directory"
        exit 1
    fi
fi

echo "Running: $RUNNER luau run --libs=$LIBS_DIR --runtime=$RUNTIME $SCRIPT"
exec "$RUNNER" luau run --libs="$LIBS_DIR" --runtime="$RUNTIME" "$SCRIPT"
