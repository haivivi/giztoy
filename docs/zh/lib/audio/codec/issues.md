# Audio Codec - Known Issues

## 🟡 Minor Issues

### COD-001: Go Opus encoder buffer allocation per call

**File:** `go/pkg/audio/codec/opus/encoder.go:95`

**Description:**  
Encode allocates a new 4000-byte buffer on every call:

```go
buf := make([]byte, 4000)
```

**Impact:** GC pressure for high-frequency encoding (e.g., 50 calls/second for 20ms frames).

**Suggestion:** Prefer existing `EncodeTo` or add a pool-backed `Encode` helper.

---

### COD-002: Go Opus decoder max samples hardcoded

**File:** `go/pkg/audio/codec/opus/decoder.go:58-59`

**Description:**  
Hardcoded max samples based on 48kHz stereo:

```go
maxSamples := 5760 * d.channels
buf := make([]int16, maxSamples)
```

**Impact:** Allocation per decode, may over-allocate for lower sample rates.

---

### COD-003: Rust codec modules lack documentation

**Description:**  
Most Rust codec functions have minimal or no documentation compared to Go.

**Impact:** Less discoverable API for Rust users.

---

### COD-004: No float PCM support in encoders

**Description:**  
Both Go and Rust opus encoders only accept int16 PCM. libopus supports float input via `opus_encode_float`.

**Impact:** Extra conversion step when working with float audio pipelines.

**Suggestion:** Add `EncodeFloat` variant.

---

## 🔵 Enhancements

### COD-005: No Opus decoder FEC support

**Description:**  
Forward Error Correction (FEC) is supported by libopus but not exposed in the API.

**Impact:** Cannot use FEC for improved packet loss resilience.

**Suggestion:** Add `decode_fec` parameter.

---

### COD-006: No MP3 VBR encoding option

**Description:**  
MP3 encoder appears to use CBR only. VBR (Variable Bit Rate) produces better quality at same file size.

**Suggestion:** Add quality-based VBR mode.

---

### COD-007: OGG container lacks Vorbis comment support

**Description:**  
OGG encoder doesn't provide helpers for adding metadata (Vorbis comments).

**Impact:** Cannot add title/artist/etc. to OGG files.

---

### COD-008: No codec capability queries

**Description:**  
Cannot query supported sample rates or channel counts before creating encoder/decoder.

**Suggestion:** Add `SupportedSampleRates() []int` etc.

---

## ⚪ Notes

### COD-009: libopus version dependency

**Description:**  
Some features may require specific libopus versions:
- 1.1: Basic functionality
- 1.2: Improved quality
- 1.3: Ambisonics support

Currently no version checking is performed.

---

### COD-010: TOC parsing is standalone

**Description:**  
TOC parsing works on raw bytes without requiring encoder/decoder. Useful for stream analysis without full codec setup.

---

### COD-011: minimp3 is header-only

**Description:**  
MP3 decoder uses minimp3, a header-only library. This:
- Simplifies build (no separate library)
- May have different behavior than reference decoder
- Single-threaded only

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| COD-001 | 🟡 Minor | Open | Go Opus |
| COD-002 | 🟡 Minor | Open | Go Opus |
| COD-003 | 🟡 Minor | Open | Rust |
| COD-004 | 🟡 Minor | Open | Both |
| COD-005 | 🔵 Enhancement | Open | Both |
| COD-006 | 🔵 Enhancement | Open | Both |
| COD-007 | 🔵 Enhancement | Open | Both |
| COD-008 | 🔵 Enhancement | Open | Both |
| COD-009 | ⚪ Note | N/A | Both |
| COD-010 | ⚪ Note | N/A | Both |
| COD-011 | ⚪ Note | N/A | Both |

**Overall:** Solid codec implementation. Main issues are allocation patterns and missing advanced features (FEC, float PCM, VBR).
