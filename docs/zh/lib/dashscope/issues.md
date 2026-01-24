# DashScope SDK - Known Issues

## 🟡 Minor Issues

### DS-001: Go NewClient panics on empty API key

**File:** `go/pkg/dashscope/client.go:39-41`

**Description:**  
NewClient panics instead of returning an error:

```go
func NewClient(apiKey string, opts ...Option) *Client {
    if apiKey == "" {
        panic("dashscope: API key is required")
    }
```

**Impact:** Unrecoverable error at construction time.

**Suggestion:** Return `(*Client, error)` like Rust version.

---

### DS-002: Limited to Realtime API only

**Description:**  
SDK only implements Realtime API. Text/Chat APIs require separate OpenAI-compatible SDK.

**Impact:** Users need two SDKs for full DashScope usage.

**Note:** This is intentional design choice - text APIs are OpenAI-compatible.

---

### DS-003: No HTTP API implementation

**Description:**  
No HTTP client for non-realtime operations (file upload, app calls, etc.).

**Suggestion:** Add HTTP service for app/agent API calls.

---

### DS-004: Video input support limited

**Description:**  
Qwen3-Omni-Flash supports video input, but SDK support may be incomplete.

**Status:** ⚠️ Needs verification.

---

## 🔵 Enhancements

### DS-005: No automatic reconnection

**Description:**  
WebSocket sessions don't auto-reconnect on disconnection.

**Suggestion:** Add reconnection with backoff for long-running sessions.

---

### DS-006: No audio transcoding

**Description:**  
Audio must be in correct format (PCM16/PCM24). No built-in transcoding.

**Suggestion:** Add optional audio format conversion.

---

### DS-007: No VAD (Voice Activity Detection) integration

**Description:**  
Manual audio buffer management. No built-in VAD for automatic speech detection.

**Suggestion:** Integrate with `audio/pcm` for silence detection.

---

### DS-008: Missing tool call examples

**Description:**  
Function/tool calling is supported but not well documented with examples.

---

## ⚪ Notes

### DS-009: Clean WebSocket event model

**Description:**  
Both Go and Rust implement clean event-based model matching OpenAI Realtime API patterns. This is well-designed.

---

### DS-010: Model constants provided

**Description:**  
Both SDKs provide model name constants:

```go
const ModelQwenOmniTurboRealtimeLatest = "qwen-omni-turbo-realtime-latest"
```

Good for discoverability and avoiding typos.

---

### DS-011: Workspace support

**Description:**  
Both SDKs support workspace isolation via `WithWorkspace()` option:

```go
client := dashscope.NewClient(apiKey, dashscope.WithWorkspace("ws-xxx"))
```

Useful for enterprise environments.

---

### DS-012: International endpoint support

**Description:**  
SDKs support both China and international endpoints:
- China: `wss://dashscope.aliyuncs.com/...`
- International: `wss://dashscope-intl.aliyuncs.com/...`

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| DS-001 | 🟡 Minor | Open | Go Client |
| DS-002 | 🟡 Minor | Note | Both |
| DS-003 | 🟡 Minor | Open | Both |
| DS-004 | 🟡 Minor | Open | Both |
| DS-005 | 🔵 Enhancement | Open | Both |
| DS-006 | 🔵 Enhancement | Open | Both |
| DS-007 | 🔵 Enhancement | Open | Both |
| DS-008 | 🔵 Enhancement | Open | Both |
| DS-009 | ⚪ Note | N/A | Both |
| DS-010 | ⚪ Note | N/A | Both |
| DS-011 | ⚪ Note | N/A | Both |
| DS-012 | ⚪ Note | N/A | Both |

**Overall:** Focused SDK for Realtime API with clean design. Main limitation is narrow scope (Realtime only), which is intentional since text APIs are OpenAI-compatible. Both Go and Rust implementations are feature-complete for their scope.
