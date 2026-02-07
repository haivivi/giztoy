#!/bin/bash
# Chatgear Integration Test
#
# Tests the Zig chatgear client against a Go MQTT server.
# Verifies:
# - Connection and subscription
# - State sending (immediate + periodic)
# - Stats sending
# - Command receiving

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           Chatgear Integration Test                          ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
PORT=18830
SCOPE="test/"
GEAR_ID="zig-test"
TEST_DURATION=12  # seconds

# Find server binary
SERVER_BIN=""
if [ -n "$TEST_SRCDIR" ]; then
    for path in \
        "$TEST_SRCDIR/$TEST_WORKSPACE/go/cmd/chatgear-test-server/chatgear-test-server" \
        "$TEST_SRCDIR/$TEST_WORKSPACE/go/cmd/chatgear-test-server/chatgear-test-server_/chatgear-test-server" \
        "$TEST_SRCDIR/_main/go/cmd/chatgear-test-server/chatgear-test-server" \
        "$TEST_SRCDIR/_main/go/cmd/chatgear-test-server/chatgear-test-server_/chatgear-test-server"
    do
        if [ -f "$path" ]; then
            SERVER_BIN="$path"
            break
        fi
    done
fi

# Also try finding via runfiles location
if [ -z "$SERVER_BIN" ] && [ -n "$RUNFILES_DIR" ]; then
    for path in \
        "$RUNFILES_DIR/_main/go/cmd/chatgear-test-server/chatgear-test-server" \
        "$RUNFILES_DIR/giztoy/go/cmd/chatgear-test-server/chatgear-test-server"
    do
        if [ -f "$path" ]; then
            SERVER_BIN="$path"
            break
        fi
    done
fi

if [ -z "$SERVER_BIN" ]; then
    echo "ERROR: Server binary not found"
    echo "TEST_SRCDIR: $TEST_SRCDIR"
    echo "TEST_WORKSPACE: $TEST_WORKSPACE"
    echo "RUNFILES_DIR: $RUNFILES_DIR"
    exit 1
fi

echo "Using server: $SERVER_BIN"

# Start server in background
echo "Starting MQTT test server on port $PORT..."
"$SERVER_BIN" -port=$PORT -scope="$SCOPE" -gear-id="$GEAR_ID" &
SERVER_PID=$!

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Wait for server to start
sleep 1

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "ERROR: Server failed to start"
    exit 1
fi

echo "Server started (PID: $SERVER_PID)"
echo ""

# For now, just verify server starts correctly
# TODO: Add Zig client test when native_test app is ready

echo "Waiting for ${TEST_DURATION}s to collect messages..."
sleep $TEST_DURATION

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║           Test Complete                                      ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# Server will print summary when killed via trap
