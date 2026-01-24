# CLI Package - Known Issues

## 🟡 Minor Issues

### CLI-001: Rust missing output formats

**Description:**  
Go supports 4 output formats: `yaml`, `json`, `table`, `raw`.
Rust only supports: `yaml`, `json`.

**Impact:** Rust CLI tools cannot output raw binary to stdout or table format.

**Suggestion:** Add `Raw` and `Table` format support to Rust.

---

### CLI-002: Rust missing print helpers

**Description:**  
Go has multiple print helpers with icons:
- `PrintSuccess` (✓)
- `PrintError`
- `PrintInfo` (ℹ)
- `PrintWarning` (⚠)
- `PrintVerbose`

Rust only has `print_verbose`.

**Impact:** Inconsistent user experience between Go and Rust CLIs.

**Suggestion:** Add print helper functions to Rust.

---

### CLI-003: Config file permissions

**File:** `go/pkg/cli/config.go:143`

**Description:**  
Config file is created with `0600` permissions (owner read/write only), which is good. But the config directory is created with `0755` (world-readable).

```go
os.MkdirAll(dir, 0755)  // Directory readable by others
os.WriteFile(c.configPath, data, 0600)  // File only owner
```

**Impact:** Directory structure is visible to other users, though file content is protected.

**Suggestion:** Consider `0700` for the config directory.

---

### CLI-004: Go Context.Extra returns empty string for missing keys

**File:** `go/pkg/cli/config.go:224-228`

**Description:**  
`GetExtra` returns empty string `""` for missing keys, making it impossible to distinguish between "key exists with empty value" and "key doesn't exist".

```go
func (ctx *Context) GetExtra(key string) string {
    if ctx.Extra == nil {
        return ""
    }
    return ctx.Extra[key]  // Returns "" for missing key
}
```

**Impact:** Cannot differentiate missing vs empty extra values.

**Suggestion:** Add `HasExtra(key string) bool` or return `(string, bool)`.

---

## 🔵 Enhancements

### CLI-005: No config file locking

**Description:**  
Neither Go nor Rust implementation locks the config file during read/write operations. Concurrent CLI processes could corrupt the config.

**Suggestion:** Implement file locking for Save operations.

---

### CLI-006: No config validation

**Description:**  
Config is loaded without validation. Invalid URLs, negative timeouts, etc. are not detected until runtime errors occur.

**Suggestion:** Add `Validate() error` method to Config/Context.

---

### CLI-007: Missing config migration

**Description:**  
No mechanism to handle config format changes between versions. If schema changes, old configs may fail to load.

**Suggestion:** Add version field and migration support.

---

### CLI-008: No environment variable support

**Description:**  
API keys and other credentials must be stored in config file. No support for environment variable overrides.

**Example:**
```bash
# Desired behavior
export MINIMAX_API_KEY="sk-..."
minimax chat "Hello"  # Uses env var instead of config
```

**Suggestion:** Add env var lookup with config fallback.

---

## ⚪ Notes

### CLI-009: Different YAML libraries

**Description:**  
- Go: Uses `github.com/goccy/go-yaml`
- Rust: Uses `serde_yaml`

Both produce compatible output but may have minor formatting differences.

---

### CLI-010: MaskAPIKey behavior for short keys

**Description:**  
Both implementations mask entire key if length <= 8:

```go
if len(key) <= 8 {
    return strings.Repeat("*", len(key))
}
```

This means very short keys (e.g., "test") show as "****" with no visible characters.

---

### CLI-011: Paths use dirs crate in Rust

**Description:**  
- Go: Uses `os.UserHomeDir()` (stdlib)
- Rust: Uses `dirs::home_dir()` (external crate)

Both handle cross-platform home directory detection correctly.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| CLI-001 | 🟡 Minor | Open | Rust Output |
| CLI-002 | 🟡 Minor | Open | Rust Print |
| CLI-003 | 🟡 Minor | Open | Go Config |
| CLI-004 | 🟡 Minor | Open | Go Context |
| CLI-005 | 🔵 Enhancement | Open | Both |
| CLI-006 | 🔵 Enhancement | Open | Both |
| CLI-007 | 🔵 Enhancement | Open | Both |
| CLI-008 | 🔵 Enhancement | Open | Both |
| CLI-009 | ⚪ Note | N/A | Both |
| CLI-010 | ⚪ Note | N/A | Both |
| CLI-011 | ⚪ Note | N/A | Rust |

**Overall:** Functional CLI utilities. Main gaps are feature parity between Go and Rust implementations.
