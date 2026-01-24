# Audio PCM - Known Issues

## 🟡 Minor Issues

### PCM-001: Format panics on invalid values

**File:** `go/pkg/audio/pcm/pcm.go:36-38`

**Description:**  
All Format methods panic if the format value is invalid:

```go
func (f Format) SampleRate() int {
    switch f {
    case L16Mono16K:
        return 16000
    // ...
    }
    panic("pcm: invalid audio type")
}
```

**Impact:** Runtime panic if format is incorrectly initialized.

**Suggestion:** Return `(int, error)` or add validation method.

---

### PCM-002: SilenceChunk uses global buffer

**File:** `go/pkg/audio/pcm/pcm.go:177`

**Description:**  
SilenceChunk shares a global 32KB zero buffer:

```go
var emptyBytes [32000]byte
```

**Impact:**  
No functional issue; `WriteTo` loops and handles any length. Shared buffer avoids allocations.

---

### PCM-003: Mixer notifyWrite creates goroutine per call

**File:** `go/pkg/audio/pcm/mixer.go:391-405`

**Description:**  
Each write notification spawns a goroutine:

```go
func (mx *Mixer) notifyWrite() {
    go func() {
        // ...
    }()
}
```

**Impact:** Goroutine creation overhead for every track write.

**Suggestion:** Use dedicated notification channel without goroutine.

---

### PCM-004: Mixer uses unsafe pointer casting

**File:** `go/pkg/audio/pcm/mixer.go:226`

**Description:**  
Converts between byte slice and int16 slice using unsafe:

```go
i16 := unsafe.Slice((*int16)(unsafe.Pointer(&p[0])), len(p)/2)
```

**Risk:** Assumes little-endian, may have alignment issues.

---

### PCM-005: Track buffer allocation per read

**File:** `go/pkg/audio/pcm/mixer.go:341`

**Description:**  
Allocates temporary buffer for each track read:

```go
trackBuf := make([]byte, len(p))
```

**Impact:** GC pressure during mixing.

**Suggestion:** Use per-track persistent buffer or sync.Pool.

---

### PCM-006: Rust Format re-export is confusing

**File:** `rust/audio/src/pcm/format.rs:7`

**Description:**  
Format is defined in `resampler` but re-exported by `pcm`:

```rust
pub use crate::resampler::format::Format;
```

**Impact:** Import path confusion.

---

## 🔵 Enhancements

### PCM-007: No stereo format support

**Description:**  
Only mono formats are predefined. Stereo is common for music.

**Suggestion:** Add `L16Stereo16K`, etc.

---

### PCM-008: No 24-bit or 32-bit PCM

**Description:**  
Only 16-bit depth supported. Professional audio uses 24/32-bit.

**Suggestion:** Add higher bit depth variants.

---

### PCM-009: No channel downmix in mixer

**Description:**  
Mixer doesn't handle mixing stereo sources to mono output or vice versa.

**Suggestion:** Add automatic channel conversion.

---

### PCM-010: No mixer input resampling

**Description:**  
All tracks must match mixer output sample rate. No automatic resampling.

**Suggestion:** Auto-resample tracks that don't match output format.

---

### PCM-011: No track volume ramping

**Description:**  
Gain changes apply instantly, potentially causing clicks.

**Suggestion:** Add smooth gain transition over configurable duration.

---

## ⚪ Notes

### PCM-012: Float32 internal mixing

**Description:**  
Mixer converts int16→float32→int16 internally for headroom during mixing. This is standard practice but involves precision loss.

---

### PCM-013: Mixer silence gap behavior

**Description:**  
When `WithSilenceGap` is set:
- Mixer tracks continuous silence duration
- Closes after gap threshold
- Initial silence counts from gap to avoid leading silence

Complex logic that could be confusing.

---

### PCM-014: AtomicFloat32 implementation

**Description:**  
Both Go and Rust implement atomic float32 via bit reinterpretation:

```go
func (a *AtomicFloat32) Load() float32 {
    return math.Float32frombits(a.v.Load())
}
```

Standard technique, but worth documenting.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| PCM-001 | 🟡 Minor | Open | Go Format |
| PCM-002 | ⚪ Note | N/A | Go Silence |
| PCM-003 | 🟡 Minor | Open | Go Mixer |
| PCM-004 | 🟡 Minor | Open | Go Mixer |
| PCM-005 | 🟡 Minor | Open | Go Mixer |
| PCM-006 | 🟡 Minor | Open | Rust |
| PCM-007 | 🔵 Enhancement | Open | Both |
| PCM-008 | 🔵 Enhancement | Open | Both |
| PCM-009 | 🔵 Enhancement | Open | Both |
| PCM-010 | 🔵 Enhancement | Open | Both |
| PCM-011 | 🔵 Enhancement | Open | Both |
| PCM-012 | ⚪ Note | N/A | Go |
| PCM-013 | ⚪ Note | N/A | Go |
| PCM-014 | ⚪ Note | N/A | Both |

**Overall:** Functional PCM handling and mixing. Main concerns are allocation patterns and missing format variants.
