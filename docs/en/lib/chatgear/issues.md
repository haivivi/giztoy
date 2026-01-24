# chatgear - Known Issues

## 🟡 Minor Issues

### CG-001: Go ReadNFCTag equality ignores tag data changes

**File:** `go/pkg/chatgear/stats.go`

**Description:**
`ReadNFCTag.Equal` compares only tag UIDs. If a tag's payload or metadata
changes but the UID remains the same, the merge logic will treat it as unchanged.

**Impact:**
Telemetry updates can be silently skipped.

**Suggestion:**
Include additional fields (e.g., `RawData`, `DataFormat`, `UpdateAt`) in equality
or document UID-only matching as a deliberate choice.

---

### CG-002: Rust SessionCommandEvent swallows serialization errors

**File:** `rust/chatgear/src/command.rs`

**Description:**
`SessionCommandEvent::new` uses `serde_json::to_value(cmd)` and replaces errors
with `Value::Null`, losing the original error context.

**Impact:**
Serialization failures are silently ignored, making debugging difficult.

**Suggestion:**
Return a `Result` or log/report serialization failures explicitly.

---

### CG-003: Go Pipe connections can block indefinitely on backpressure

**File:** `go/pkg/chatgear/conn_pipe.go`

**Description:**
`NewPipe` uses bounded channels. If the receiver stops reading, senders will
block in `SendOpusFrames` / `SendState` / `SendStats` without a timeout unless
the caller provides a cancellable context.

**Impact:**
Potential goroutine leaks in tests or in-process usage.

**Suggestion:**
Document this behavior and recommend context timeouts for pipe usage.
