# Audio Package - Known Issues

## 🟠 Major Issues

### AUD-001: Go Mixer uses unsafe pointer casting

**File:** `go/pkg/audio/pcm/mixer.go:226`

**Description:**  
The mixer uses `unsafe.Slice` and `unsafe.Pointer` to cast between `[]byte` and `[]int16`:

```go
i16 := unsafe.Slice((*int16)(unsafe.Pointer(&p[0])), len(p)/2)
```

**Risk:**
- Platform-dependent endianness (assumes little-endian)
- Potential undefined behavior if buffer alignment is wrong

**Impact:** May produce incorrect audio on big-endian systems.

**Suggestion:** Add explicit little-endian encoding/decoding or document platform requirements.

---

### AUD-002: Rust opusrt missing OGG Reader/Writer

**Description:**  
Go opusrt has `OggReader` and `OggWriter` for reading/writing Opus in OGG containers. Rust implementation is missing these.

**Impact:** Cannot read/write Opus files in OGG format in Rust.

**Status:** ⚠️ Partial implementation.

---

### AUD-003: Rust missing portaudio module

**Description:**  
Go has `audio/portaudio` for audio device I/O. Rust has no equivalent.

**Impact:** Cannot capture/play audio from hardware devices in Rust.

**Status:** ❌ Not implemented.

---

## 🟡 Minor Issues

### AUD-004: Go Format panics on invalid value

**File:** `go/pkg/audio/pcm/pcm.go:36-38`

**Description:**  
`Format.SampleRate()`, `Channels()`, `Depth()` all panic on invalid format:

```go
func (f Format) SampleRate() int {
    switch f {
    // ...
    }
    panic("pcm: invalid audio type")
}
```

**Impact:** Runtime panic instead of error return.

**Suggestion:** Return `(int, error)` or use `MustXxx` naming convention for panicking versions.

---

### AUD-005: Go SilenceChunk uses fixed global buffer

**File:** `go/pkg/audio/pcm/pcm.go:177`

**Description:**  
Uses a shared fixed-size zero buffer and loops for long durations.

**Impact:** None functionally; avoids repeated allocations.

**Status:** Not a bug. Keep as implementation note only.

### AUD-006: Go Opus encoder max frame size hardcoded

**File:** `go/pkg/audio/codec/opus/encoder.go:95`

**Description:**  
Encode function allocates fixed 4000 byte buffer:

```go
buf := make([]byte, 4000)
```

**Impact:** Allocation on every encode call.

**Suggestion:** Use buffer pool or allow caller to provide buffer.

---

### AUD-007: Rust Format re-export from resampler is confusing

**File:** `rust/audio/src/pcm/format.rs:7`

**Description:**  
`pcm::Format` is actually re-exported from `resampler::Format`:

```rust
pub use crate::resampler::format::Format;
```

**Impact:** Confusing import paths, circular dependency appearance.

**Suggestion:** Define Format once at top level and import in both modules.

---

### AUD-008: Go mixer notifyWrite spawns goroutine every call

**File:** `go/pkg/audio/pcm/mixer.go:391-405`

**Description:**  
`notifyWrite()` spawns a new goroutine each time:

```go
func (mx *Mixer) notifyWrite() {
    go func() {
        // ...
    }()
}
```

**Impact:** Goroutine overhead for every write notification.

**Suggestion:** Use single dedicated notification goroutine or avoid goroutine.

---

## 🔵 Enhancements

### AUD-009: No stereo format support in predefined formats

**Description:**  
Only mono formats are predefined (`L16Mono16K`, etc.). No stereo formats.

**Suggestion:** Add `L16Stereo16K`, `L16Stereo24K`, `L16Stereo48K`.

---

### AUD-010: No 8-bit or 24-bit PCM support

**Description:**  
Only 16-bit PCM is supported. Some audio sources use 8-bit (low quality) or 24-bit (high quality).

**Suggestion:** Add format variants for different bit depths.

---

### AUD-011: Resampler quality not configurable

**File:** `go/pkg/audio/resampler/soxr.go:52`

**Description:**  
Quality is hardcoded to `SOXR_HQ`:

```go
qSpec := C.soxr_quality_spec(C.SOXR_HQ, 0)
```

**Impact:** Cannot trade quality for performance when needed.

**Suggestion:** Add quality parameter to New().

---

### AUD-012: No WAV file support

**Description:**  
No utilities for reading/writing WAV files, only raw PCM.

**Suggestion:** Add WAV header parsing/writing for file I/O.

---

## ⚪ Notes

### AUD-013: CGO/FFI dependency complexity

**Description:**  
Both Go and Rust rely heavily on CGO/FFI for native codec libraries. This adds:
- Build complexity (pkg-config, Bazel rules)
- Platform-specific issues
- Memory management concerns

**Status:** Necessary for performance, but increases maintenance burden.

---

### AUD-014: Mixer uses float32 internally

**Description:**  
Mixer converts int16 PCM to float32 for mixing, then back to int16:

```go
// int16 → float32
s := float32(trackI16[i])
// ... mix ...
// float32 → int16
i16[i] = int16(t * 32767)
```

**Impact:** Slight precision loss during mixing, but standard practice.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| AUD-001 | 🟠 Major | Open | Go Mixer |
| AUD-002 | 🟠 Major | Open | Rust opusrt |
| AUD-003 | 🟠 Major | Open | Rust |
| AUD-004 | 🟡 Minor | Open | Go Format |
| AUD-005 | ⚪ Note | N/A | Go SilenceChunk |
| AUD-006 | 🟡 Minor | Open | Go Opus |
| AUD-007 | 🟡 Minor | Open | Rust Format |
| AUD-008 | 🟡 Minor | Open | Go Mixer |
| AUD-009 | 🔵 Enhancement | Open | Both |
| AUD-010 | 🔵 Enhancement | Open | Both |
| AUD-011 | 🔵 Enhancement | Open | Go |
| AUD-012 | 🔵 Enhancement | Open | Both |
| AUD-013 | ⚪ Note | N/A | Both |
| AUD-014 | ⚪ Note | N/A | Go |

**Overall:** Functional audio processing with significant native library integration. Main gaps are Rust parity (opusrt OGG, portaudio) and some unsafe code patterns.
