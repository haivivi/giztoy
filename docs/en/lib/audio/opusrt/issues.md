# Audio OpusRT - Known Issues

## 🟠 Major Issues

### ORT-001: Rust missing RealtimeBuffer

**Description:**  
Go has `RealtimeBuffer` for real-time playback simulation. Rust implementation is missing this.

**Impact:** Cannot simulate real-time audio playback in Rust without manual timing logic.

**Status:** ❌ Not implemented.

---

### ORT-002: Rust missing OGG Reader/Writer

**Description:**  
Go has `OggReader` and `OggWriter` for reading/writing Opus in OGG containers. Rust is missing these.

**Impact:** Cannot read/write Opus files in standard OGG format in Rust.

**Status:** ❌ Not implemented.

---

## 🟡 Minor Issues

### ORT-003: Buffer drops old frames silently

**File:** `go/pkg/audio/opusrt/buffer.go:136-141`

**Description:**  
When buffer exceeds max duration, oldest frames are dropped with only debug logging:

```go
for buf.buffered > buf.duration() {
    slog.Debug("opusrt: remove frame", ...)
    buf.pop()
}
```

**Impact:** Caller may not know data was lost.

**Suggestion:** Return dropped count or emit metric.

---

### ORT-004: RealtimeBuffer spawns goroutine unconditionally

**File:** `go/pkg/audio/opusrt/realtime.go:43`

**Description:**  
`RealtimeFrom` always starts a background goroutine:

```go
func RealtimeFrom(buf *Buffer) *RealtimeBuffer {
    // ...
    go rtb.pull()
    return rtb
}
```

**Impact:**
- Goroutine leak if RealtimeBuffer not properly closed
- Cannot use lazy initialization

---

### ORT-005: Timestamp epsilon is magic number

**File:** `go/pkg/audio/opusrt/realtime.go:19`

**Description:**  
The 2ms epsilon is hardcoded without clear documentation:

```go
const timestampEpsilon = 2
```

**Impact:** May not be appropriate for all use cases.

**Suggestion:** Make configurable or document reasoning better.

---

### ORT-006: Buffer.Write ignores Append error

**File:** `go/pkg/audio/opusrt/buffer.go:156`

**Description:**  
The Write method ignores the error from Append:

```go
func (buf *Buffer) Write(stamped []byte) (int, error) {
    // ...
    buf.Append(frame.Clone(), ts)  // Error ignored!
    return len(stamped), nil
}
```

**Impact:** ErrDisorderedPacket is silently ignored.

---

### ORT-007: OggWriter requires manual Close

**Description:**  
OggWriter must be closed to write the final OGG page. Not closing results in corrupted file.

**Impact:** Easy to forget, no defer pattern works well.

**Suggestion:** Document prominently or add finalizer.

---

## 🔵 Enhancements

### ORT-008: No FEC (Forward Error Correction) support

**Description:**  
Opus FEC can recover some lost data without PLC. Current implementation doesn't use this.

**Suggestion:** Add FEC extraction from subsequent frames.

---

### ORT-009: No statistics/metrics

**Description:**  
No way to get buffer statistics:
- Packets received
- Packets lost
- Jitter measurements
- Buffer fullness

**Suggestion:** Add Stats() method.

---

### ORT-010: No adaptive buffer sizing

**Description:**  
Buffer duration is fixed at creation. Real-world systems often adapt buffer size based on jitter.

**Suggestion:** Add adaptive buffering option.

---

### ORT-011: No RTCP support

**Description:**  
No support for RTCP (Real-time Transport Control Protocol) which provides feedback for real-time streams.

**Suggestion:** Consider RTCP integration for WebRTC use cases.

---

## ⚪ Notes

### ORT-012: Heap-based ordering

**Description:**  
Buffer uses Go's container/heap for timestamp ordering:
- O(log n) insert
- O(log n) pop
- Memory for heap structure

This is appropriate for jitter buffer sizes (typically < 1000 frames).

---

### ORT-013: StampedFrame format

**Description:**  
Binary format:
```
[0:8]  - Timestamp (int64, big-endian)
[8:]   - Opus frame data
```

This is a custom format, not a standard protocol.

---

### ORT-014: Loss threshold in RealtimeBuffer

**Description:**  
RealtimeBuffer has complex loss handling with thresholds:

```go
const (
    pullInterval  = 20
    lossStep      = pullInterval * time.Millisecond
    lossThreshold = 10 * lossStep  // 200ms
)
```

Designed to avoid spurious loss reports during initial buffering.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| ORT-001 | 🟠 Major | Open | Rust |
| ORT-002 | 🟠 Major | Open | Rust |
| ORT-003 | 🟡 Minor | Open | Go Buffer |
| ORT-004 | 🟡 Minor | Open | Go Realtime |
| ORT-005 | 🟡 Minor | Open | Go |
| ORT-006 | 🟡 Minor | Open | Go Buffer |
| ORT-007 | 🟡 Minor | Open | Go OGG |
| ORT-008 | 🔵 Enhancement | Open | Both |
| ORT-009 | 🔵 Enhancement | Open | Both |
| ORT-010 | 🔵 Enhancement | Open | Both |
| ORT-011 | 🔵 Enhancement | Open | Both |
| ORT-012 | ⚪ Note | N/A | Go |
| ORT-013 | ⚪ Note | N/A | Both |
| ORT-014 | ⚪ Note | N/A | Go |

**Overall:** Go implementation is comprehensive. Rust implementation is significantly incomplete, missing RealtimeBuffer and OGG support which are critical for real-world use.
