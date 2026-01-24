# mqtt0 - Known Issues

## 🟠 Major Issues

### MQTT0-001: Go Client read path is not demultiplexed

**File:** `go/pkg/mqtt0/client.go`

**Description:**
`Subscribe`, `Unsubscribe`, and other request/response operations read directly
from the same stream as `Recv()`. If callers run `Recv()` concurrently with
`Subscribe()` or `Unsubscribe()`, whichever acquires `readMu` first may consume
packets that belong to the other operation, causing unexpected packet errors.

**Impact:**
Hard-to-debug race between subscription changes and inbound message handling.

**Suggestion:**
Introduce a single read loop with protocol demuxing, or document that
`Recv()` must not run concurrently with subscribe/unsubscribe calls.

---

## 🟡 Minor Issues

### MQTT0-002: Go Broker drops messages on backpressure

**File:** `go/pkg/mqtt0/broker.go`

**Description:**
The broker uses a bounded channel for each client. When the channel is full,
messages are dropped with a debug log.

**Impact:**
Message loss under bursty load beyond QoS 0 expectations; may surprise users.

**Suggestion:**
Document the drop behavior clearly or make buffer size configurable.

---

### MQTT0-003: Rust WebSocket transport requires special handling

**File:** `rust/mqtt0/src/transport.rs`

**Description:**
`Transport::WebSocket` implements `AsyncRead/AsyncWrite` by returning
`Unsupported` errors. If code treats `Transport` uniformly, WebSocket
connections will fail at runtime.

**Impact:**
Surprising runtime errors for WebSocket clients if not handled explicitly.

**Suggestion:**
Expose dedicated websocket read/write APIs or document the required handling.
