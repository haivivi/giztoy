# Audio Resampler - Known Issues

## 🟡 Minor Issues

### RSM-001: Quality level hardcoded

**File:** `go/pkg/audio/resampler/soxr.go:52`

**Description:**  
Quality is hardcoded to `SOXR_HQ`:

```go
qSpec := C.soxr_quality_spec(C.SOXR_HQ, 0)
```

**Impact:** Cannot trade quality for performance when needed.

**Suggestion:** Add quality parameter to `New()`.

---

### RSM-002: Go Read not concurrency-safe

**File:** `go/pkg/audio/resampler/soxr.go:90-150`

**Description:**  
`Read` uses internal buffer and mutex, but documentation doesn't clearly state thread-safety limitations.

**Impact:** Potential confusion about concurrent use.

**Suggestion:** Add explicit thread-safety documentation.

---

### RSM-003: Rust API differs significantly from Go

**Description:**  
- Go: `io.Reader` wrapper with `Read()` method
- Rust: Direct `process()` with byte slices

**Impact:** Code not easily portable between languages.

---

### RSM-004: Stereo/mono conversion before soxr

**File:** `go/pkg/audio/resampler/soxr.go:228-278`

**Description:**  
Channel conversion is done separately from sample rate conversion:

```go
if r.srcFmt.Stereo && !r.dstFmt.Stereo {
    return stereoToMono(r.readBuf[:rn]), err
}
```

**Impact:**
- Two-pass processing for stereo→mono + rate change
- Not using libsoxr's built-in channel handling

**Suggestion:** Use soxr's multi-channel support directly.

---

### RSM-005: Go stereoToMono loses precision

**File:** `go/pkg/audio/resampler/soxr.go:252-264`

**Description:**  
Averages L+R channels with integer division:

```go
m := int16((int32(l) + int32(r)) / 2)
```

**Impact:** Potential off-by-one errors, always rounds down.

**Suggestion:** Use rounding division: `(l + r + 1) / 2`.

---

## 🔵 Enhancements

### RSM-006: No async/streaming API

**Description:**  
Both implementations are synchronous. No async/await support in Rust.

**Suggestion:** Add async interface for non-blocking resampling.

---

### RSM-007: No passband/stopband configuration

**Description:**  
Cannot configure filter characteristics:
- Passband ripple
- Stopband attenuation
- Transition bandwidth

**Suggestion:** Expose libsoxr quality parameters.

---

### RSM-008: No float32 PCM support

**Description:**  
Only int16 PCM is supported. Many audio pipelines use float32 internally.

**Suggestion:** Add float32 I/O format option.

---

### RSM-009: No resampling ratio limits

**Description:**  
No validation of extreme resampling ratios (e.g., 8000→192000).

**Impact:** Quality degradation or memory issues with extreme ratios.

**Suggestion:** Add ratio validation or warnings.

---

## ⚪ Notes

### RSM-010: libsoxr memory management

**Description:**  
libsoxr allocates internal buffers based on:
- Quality level
- Input/output sample rates
- Number of channels

Memory is released only on `soxr_delete`. Go/Rust wrappers handle this in Close/Drop.

---

### RSM-011: Sample alignment handling

**Description:**  
Both implementations ensure reads are sample-aligned:
- Go: `sampleReader` wrapper
- Rust: `SampleReader` struct

This handles partial reads from underlying sources.

---

### RSM-012: Format definition location

**Description:**  
`Format` is defined in `resampler` module but re-exported by `pcm` module. This could be confusing:

```rust
// Both work:
use giztoy_audio::resampler::Format;
use giztoy_audio::pcm::Format;
```

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| RSM-001 | 🟡 Minor | Open | Both |
| RSM-002 | 🟡 Minor | Open | Go |
| RSM-003 | 🟡 Minor | Open | Both |
| RSM-004 | 🟡 Minor | Open | Go |
| RSM-005 | 🟡 Minor | Open | Go |
| RSM-006 | 🔵 Enhancement | Open | Both |
| RSM-007 | 🔵 Enhancement | Open | Both |
| RSM-008 | 🔵 Enhancement | Open | Both |
| RSM-009 | 🔵 Enhancement | Open | Both |
| RSM-010 | ⚪ Note | N/A | Both |
| RSM-011 | ⚪ Note | N/A | Both |
| RSM-012 | ⚪ Note | N/A | Rust |

**Overall:** Functional resampling with good quality defaults. Main issues are API differences and hardcoded quality settings.
