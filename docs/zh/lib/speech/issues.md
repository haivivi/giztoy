# speech - Known Issues

## 🟡 Minor Issues

### SPT-001: Go Revoice has no cancellation propagation

**File:** `go/pkg/speech/tts.go`

**Description:**
`Revoice` spawns a goroutine that copies the entire input speech into an
`io.Pipe`, but the goroutine is not tied to the caller's context. If the
synthesizer returns early or the context is canceled, the copy goroutine may
continue doing work until completion.

**Impact:**
Wasted CPU or lingering goroutines on early cancellation.

**Suggestion:**
Honor context cancellation inside the copy loop or use a pipe that is closed
when `ctx.Done()` fires.

---

### SPT-002: Rust ASR ignores full-transcribe implementations

**File:** `rust/speech/src/asr.rs`

**Description:**
`ASR::transcribe` always falls back to the streaming path, even though a
`Transcriber` trait exists for full transcription.

**Impact:**
Backends that can provide a more efficient full-transcribe path cannot use it.

**Suggestion:**
Detect and use `Transcriber` implementations when available.
