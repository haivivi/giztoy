# chatgear/transport - Known Issues

## 🟡 Minor Issues

### CGT-001: Go Opus encode options setter is not in interface

**File:** `go/pkg/chatgear/conn.go`, `go/pkg/chatgear/conn_pipe.go`

**Description:**
`DownlinkTx` exposes `OpusEncodeOptions()` but there is no interface method to
set or update the options. `PipeServerConn` provides `SetOpusEncodeOptions`,
but it is not part of the public interface.

**Impact:**
Callers holding a `DownlinkTx` cannot configure encode options generically.

**Suggestion:**
Add a setter to `DownlinkTx` or move encode options into a separate config
interface.

---

### CGT-002: Rust OpusEncodeOptions not wired to transport traits

**File:** `rust/chatgear/src/conn.rs`

**Description:**
`OpusEncodeOptions` exists but is not exposed through `DownlinkTx` or other
transport traits.

**Impact:**
Encoding parameters cannot be discovered or negotiated through the transport
API.

**Suggestion:**
Add accessors to `DownlinkTx` or document external negotiation.
