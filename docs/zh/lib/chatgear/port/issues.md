# chatgear/port - Known Issues

## 🟡 Minor Issues

### CGP-001: WiFi security not exposed by port command API

**File:** `go/pkg/chatgear/port.go`, `rust/chatgear/src/port.rs`

**Description:**
`SetWifi` command contains `security`, but `ServerPortTx::SetWifi` only accepts
`ssid` and `password`. The security mode cannot be specified through the port
interface.

**Impact:**
Limits configuration for networks requiring explicit security types.

**Suggestion:**
Add a `security` parameter or provide a higher-level WiFi config struct.

---

### CGP-002: ClientPortRx does not surface receive errors

**File:** `go/pkg/chatgear/port.go`, `rust/chatgear/src/port.rs`

**Description:**
Go returns commands via a channel without error context; Rust returns `Option`
from `recv_*` methods. Transport errors are not exposed on the receive side.

**Impact:**
Callers cannot distinguish between clean shutdown and underlying IO errors.

**Suggestion:**
Return `(value, error)` pairs or provide an error channel/stream.
