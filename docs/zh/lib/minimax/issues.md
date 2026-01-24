# MiniMax SDK - Known Issues

## 🟡 Minor Issues

### MMX-001: Go NewClient panics on empty API key

**File:** `go/pkg/minimax/client.go:100-102`

**Description:**  
NewClient panics instead of returning an error:

```go
func NewClient(apiKey string, opts ...Option) *Client {
    if apiKey == "" {
        panic("minimax: apiKey must be non-empty")
    }
```

**Impact:** Unrecoverable error at construction time.

**Suggestion:** Return `(*Client, error)` or use builder pattern like Rust.

---

### MMX-002: Rust services created on each call

**File:** `rust/minimax/src/client.rs:91-123`

**Description:**  
Service getters create new instances each call:

```rust
pub fn speech(&self) -> SpeechService {
    SpeechService::new(self.http.clone())
}
```

**Impact:** Arc clone overhead on each service access.

**Suggestion:** Cache services or use `&self` references.

---

### MMX-003: Go streaming uses hex encoding

**File:** `go/pkg/minimax/speech.go:51-56`

**Description:**  
Audio data comes hex-encoded from API, decoded in SDK:

```go
if apiResp.Data.Audio != "" {
    audio, err := decodeHexAudio(apiResp.Data.Audio)
```

**Impact:** CPU overhead for decoding, 2x memory during decode.

**Note:** This is API design, not SDK issue, but worth documenting.

---

### MMX-004: No request timeout option

**Description:**  
Both Go and Rust SDKs don't have request-level timeout option. Go suggests using `context.WithTimeout`, Rust doesn't document timeout handling.

**Suggestion:** Add timeout option or document clearly.

---

### MMX-005: Go iter.Seq2 requires Go 1.23+

**File:** `go/pkg/minimax/speech.go:78`

**Description:**  
Streaming uses `iter.Seq2` which requires Go 1.23:

```go
func (s *SpeechService) SynthesizeStream(ctx context.Context, req *SpeechRequest) iter.Seq2[*SpeechChunk, error]
```

**Impact:** Not compatible with older Go versions.

**Note:** Modern API choice, acceptable trade-off.

---

### MMX-006: Error handling inconsistency

**Description:**  
Go uses `AsError()` helper function, Rust uses error enum matching.

**Go:**
```go
if e, ok := minimax.AsError(err); ok {
    if e.IsRateLimit() { ... }
}
```

**Rust:**
```rust
match err {
    Error::Api { status_code, .. } => { ... }
}
```

**Impact:** Different patterns between languages.

---

## 🔵 Enhancements

### MMX-007: No WebSocket TTS support

**Description:**  
Official API supports WebSocket for TTS (`/v1/t2a_ws`), but SDK only implements HTTP.

**Suggestion:** Add WebSocket-based streaming TTS for lower latency.

---

### MMX-008: No request validation

**Description:**  
No client-side validation before sending requests. Invalid parameters only fail after API call.

**Suggestion:** Add validation for known constraints (text length, model names, etc.).

---

### MMX-009: No retry backoff configuration

**Description:**  
Retry count is configurable, but backoff strategy is hardcoded.

**Suggestion:** Add configurable backoff (exponential, jitter).

---

### MMX-010: No request/response logging

**Description:**  
No built-in debug logging for API requests/responses.

**Suggestion:** Add optional logging middleware or debug mode.

---

### MMX-011: No rate limit handling

**Description:**  
Rate limit errors are returned but not automatically handled (e.g., exponential backoff, queue).

**Suggestion:** Add optional rate limit handling.

---

## ⚪ Notes

### MMX-012: Full API coverage achieved

**Description:**  
Both Go and Rust SDKs implement all documented MiniMax API endpoints:
- Text generation
- Speech synthesis (sync, stream, async)
- Voice management (list, clone, design)
- Video generation (text-to-video, image-to-video, agent)
- Image generation
- Music generation
- File management

---

### MMX-013: Async task pattern

**Description:**  
Long-running operations (video, async speech, music) use a consistent pattern:
1. Create task → returns `Task[T]`
2. Call `task.Wait()` for automatic polling
3. Or manual `task.Query()` for custom logic

This is a well-designed abstraction.

---

### MMX-014: Base URL handling

**Description:**  
Both SDKs support China and Global endpoints:
- China: `https://api.minimaxi.com`
- Global: `https://api.minimaxi.chat`

Correctly defaulting to China URL.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| MMX-001 | 🟡 Minor | Open | Go Client |
| MMX-002 | 🟡 Minor | Open | Rust Client |
| MMX-003 | 🟡 Minor | Note | Both |
| MMX-004 | 🟡 Minor | Open | Both |
| MMX-005 | 🟡 Minor | Note | Go |
| MMX-006 | 🟡 Minor | Open | Both |
| MMX-007 | 🔵 Enhancement | Open | Both |
| MMX-008 | 🔵 Enhancement | Open | Both |
| MMX-009 | 🔵 Enhancement | Open | Both |
| MMX-010 | 🔵 Enhancement | Open | Both |
| MMX-011 | 🔵 Enhancement | Open | Both |
| MMX-012 | ⚪ Note | N/A | Both |
| MMX-013 | ⚪ Note | N/A | Both |
| MMX-014 | ⚪ Note | N/A | Both |

**Overall:** Well-implemented SDK with full API coverage. Both Go and Rust implementations are feature-complete and production-ready. Minor issues are mostly design choices rather than bugs.
