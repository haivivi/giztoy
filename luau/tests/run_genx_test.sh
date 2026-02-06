#!/bin/bash
# Run a Luau genx integration test script
# Supports model loading and JSON input via file

set -e

# Running inside bazel sandbox
RUNNER="$1"
SCRIPT="$2"
LIBS_DIR="$3"
MODELS_DIR="$4"
INPUT_FILE="$5"
RUNTIME="${6:-tool}"

# In bazel sandbox, files are in runfiles directory
if [[ -d "$LIBS_DIR" ]] && [[ -d "$LIBS_DIR/haivivi" ]]; then
    : # LIBS_DIR is already correct
elif [[ -d "luau/libs/haivivi" ]]; then
    LIBS_DIR="luau/libs"
elif [[ -d "../luau/libs/haivivi" ]]; then
    LIBS_DIR="../luau/libs"
else
    SCRIPT_DIR=$(dirname "$SCRIPT")
    if [[ -d "$SCRIPT_DIR/../libs/haivivi" ]]; then
        LIBS_DIR="$SCRIPT_DIR/../libs"
    else
        echo "ERROR: Cannot find libs directory"
        exit 1
    fi
fi

# Find models directory
if [[ ! -d "$MODELS_DIR" ]]; then
    # Try common paths in bazel sandbox
    if [[ -d "testdata/models" ]]; then
        MODELS_DIR="testdata/models"
    fi
fi

echo "Running: $RUNNER --dir=$LIBS_DIR --models=$MODELS_DIR --runtime=$RUNTIME -v $SCRIPT"
echo "Models dir contents:"
ls -la "$MODELS_DIR" 2>/dev/null || echo "Cannot list models directory"

if [[ -n "$INPUT_FILE" ]] && [[ -f "$INPUT_FILE" ]]; then
    echo "Input file: $INPUT_FILE"
    cat "$INPUT_FILE"
    echo ""
    cat "$INPUT_FILE" | "$RUNNER" --dir="$LIBS_DIR" --models="$MODELS_DIR" --runtime="$RUNTIME" -v "$SCRIPT"
else
    "$RUNNER" --dir="$LIBS_DIR" --models="$MODELS_DIR" --runtime="$RUNTIME" -v "$SCRIPT"
fi
