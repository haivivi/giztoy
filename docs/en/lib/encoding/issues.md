# Encoding Package - Known Issues

## ⚪ Notes

### ENC-001: Go/Rust type structure difference

**Description:**  
Go uses type alias (`type StdBase64Data []byte`) while Rust uses newtype wrapper (`struct StdBase64Data(Vec<u8>)`).

**Impact:**  
- Go: Direct cast to `[]byte`, shares memory
- Rust: Requires `.as_bytes()` or deref coercion, owns memory

**Status:** By design - idiomatic in each language.

---

### ENC-002: Rust has more utility methods

**Description:**  
Rust implementation has additional methods not present in Go:
- `is_empty()` / `len()`
- `encode()` / `decode()` (standalone, not just JSON)
- `empty()` constructor
- `as_bytes_mut()` for mutation

**Suggestion:** Consider adding these to Go for parity.

---

### ENC-003: Error handling difference

**Description:**  
- Go: Returns `error` on unmarshal failure
- Rust: Returns `Result<T, E>` with specific error types (`base64::DecodeError`, `hex::FromHexError`)

**Impact:** Different error inspection patterns in each language.

---

## 🔵 Enhancements

### ENC-004: Missing URL-safe Base64 variant

**Description:**  
Only standard Base64 is implemented. URL-safe Base64 (`base64.URLEncoding` / `URL_SAFE`) is commonly needed for:
- JWT tokens
- URL parameters
- Filename-safe identifiers

**Suggestion:** Add `UrlBase64Data` type.

---

### ENC-005: No raw Base64 (no padding) variant

**Description:**  
Some APIs use raw Base64 without `=` padding. Neither implementation supports this variant.

**Suggestion:** Add `RawBase64Data` or add encoding options.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| ENC-001 | ⚪ Note | By design | Go/Rust |
| ENC-002 | ⚪ Note | Open | Go |
| ENC-003 | ⚪ Note | By design | Go/Rust |
| ENC-004 | 🔵 Enhancement | Open | Both |
| ENC-005 | 🔵 Enhancement | Open | Both |

**Overall:** Clean implementation with no bugs found. Minor parity differences between Go and Rust.
