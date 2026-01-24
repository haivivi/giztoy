# Jiutian - Known Issues

## 🔴 Major Issues

### JT-001: No SDK implementation

**Description:**  
No Go or Rust SDK implementation exists for Jiutian API.

**Impact:** Users must use OpenAI-compatible SDK or implement HTTP calls directly.

**Recommendation:** 
1. For chat completions: Use OpenAI SDK with custom base URL
2. For device features: Implement direct HTTP calls

---

## 🔵 Enhancements

### JT-002: Native SDK desired

**Description:**  
A native SDK would be useful for:
- Device registration/heartbeat protocols
- Token management
- Jiutian-specific error handling

**Priority:** Low - OpenAI SDK covers main use case.

---

### JT-003: Device protocol documentation only

**Description:**  
Device registration and heartbeat protocols are documented but not implemented.

**Files affected:**
- [api/device.md](./api/device.md)

---

## ⚪ Notes

### JT-004: OpenAI-compatible API

**Description:**  
Jiutian chat API is OpenAI-compatible, so existing OpenAI SDKs work:
- Go: `github.com/sashabaranov/go-openai`
- Rust: `async-openai`
- Python: `openai`

Just set custom base URL and use Jiutian token.

---

### JT-005: Access requirements

**Description:**  
Jiutian API requires:
1. IP whitelist registration
2. Product ID from management platform
3. AI token (obtained via email application)

This is documented in [api/README.md](./api/README.md).

---

### JT-006: China Mobile specific

**Description:**  
This API is specific to China Mobile's terminal intelligent agent service management platform. May not be relevant for all users of this repository.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| JT-001 | 🔴 Major | Open | Both |
| JT-002 | 🔵 Enhancement | Open | Both |
| JT-003 | 🔵 Enhancement | Open | Both |
| JT-004 | ⚪ Note | N/A | Both |
| JT-005 | ⚪ Note | N/A | Both |
| JT-006 | ⚪ Note | N/A | Both |

**Overall:** Documentation-only module without SDK implementation. OpenAI-compatible SDK recommended for chat functionality.
