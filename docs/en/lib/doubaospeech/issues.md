# Doubao Speech SDK - Known Issues

## 🟡 Minor Issues

### DBS-001: Go auth header format unusual

**File:** `go/pkg/doubaospeech/client.go:238-239`

**Description:**  
Bearer token format is `Bearer;{token}` instead of standard `Bearer {token}`:

```go
req.Header.Set("Authorization", "Bearer;"+c.config.accessToken)
```

**Impact:** Non-standard but required by Volcengine API.

**Note:** This is API requirement, not SDK issue.

---

### DBS-002: Multiple auth method complexity

**Description:**  
SDK supports 4+ authentication methods:
- API Key (`x-api-key`)
- Bearer Token (`Authorization: Bearer;`)
- V2 API Key (`X-Api-Access-Key`, `X-Api-App-Key`)
- Resource-specific fixed keys

**Impact:** Confusing for users which method to use for which service.

**Suggestion:** Add helper methods like `NewTTSClient()`, `NewRealtimeClient()` with correct defaults.

---

### DBS-003: Resource ID vs Cluster confusion

**Description:**  
V1 uses "cluster", V2 uses "resource_id" for service selection:
- V1: `WithCluster("volcano_tts")`
- V2: `WithResourceID("seed-tts-2.0")`

**Impact:** Easy to mix up, unclear which to use when.

---

### DBS-004: Rust async TTS incomplete

**Description:**  
Rust implementation for async long-text TTS may be incomplete or missing compared to Go.

**Status:** ⚠️ Needs verification.

---

### DBS-005: Rust file ASR incomplete

**Description:**  
Rust implementation for file-based ASR may be incomplete compared to Go.

**Status:** ⚠️ Needs verification.

---

### DBS-006: Fixed app keys hardcoded

**File:** `go/pkg/doubaospeech/client.go:17-24`

**Description:**  
Some V3 APIs use fixed app keys from documentation:

```go
const (
    AppKeyRealtime = "PlgvMymc7f3tQnJ6"
    AppKeyPodcast = "aGjiRDfUWi"
)
```

**Impact:** If Volcengine changes these, SDK breaks until updated.

**Note:** This is documented API behavior.

---

## 🔵 Enhancements

### DBS-007: No automatic service version selection

**Description:**  
User must manually choose between V1 and V2 services. No automatic selection based on features needed.

**Suggestion:** Add unified service that routes to correct version.

---

### DBS-008: No connection pooling documentation

**Description:**  
WebSocket connections for streaming services could benefit from pooling documentation.

---

### DBS-009: No retry for WebSocket connections

**Description:**  
HTTP requests have retry, but WebSocket connections don't auto-reconnect on failure.

**Suggestion:** Add reconnection logic for streaming sessions.

---

### DBS-010: Console API missing some endpoints

**Description:**  
Console client may not cover all management APIs available on Volcengine.

---

## ⚪ Notes

### DBS-011: Dual API version design

**Description:**  
Having both V1 (Classic) and V2 (BigModel) services in same client reflects Volcengine's actual API structure. This is intentional, not a flaw.

---

### DBS-012: Protocol module for WebSocket

**Description:**  
Both Go and Rust have a `protocol` module for WebSocket message serialization. This is well-structured for the binary protocol requirements.

---

### DBS-013: Comprehensive service coverage

**Description:**  
SDK covers nearly all Doubao Speech services:
- TTS (sync, stream, async)
- ASR (one-sentence, stream, file)
- Voice Clone
- Realtime Dialogue
- Meeting Transcription
- Podcast Synthesis
- Translation
- Media Subtitle

This is impressive coverage.

---

### DBS-014: Console uses AK/SK signature

**Description:**  
Console API uses Volcengine OpenAPI signature (HMAC-SHA256), not simple token. This is standard for Volcengine management APIs.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| DBS-001 | 🟡 Minor | Note | Go Auth |
| DBS-002 | 🟡 Minor | Open | Both |
| DBS-003 | 🟡 Minor | Open | Both |
| DBS-004 | 🟡 Minor | Open | Rust |
| DBS-005 | 🟡 Minor | Open | Rust |
| DBS-006 | 🟡 Minor | Note | Both |
| DBS-007 | 🔵 Enhancement | Open | Both |
| DBS-008 | 🔵 Enhancement | Open | Both |
| DBS-009 | 🔵 Enhancement | Open | Both |
| DBS-010 | 🔵 Enhancement | Open | Both |
| DBS-011 | ⚪ Note | N/A | Both |
| DBS-012 | ⚪ Note | N/A | Both |
| DBS-013 | ⚪ Note | N/A | Both |
| DBS-014 | ⚪ Note | N/A | Both |

**Overall:** Comprehensive SDK with excellent API coverage. Main complexity is from Volcengine's dual API version system and multiple authentication methods. Rust implementation may have some gaps compared to Go.
