# JsonTime Package - Known Issues

## 🟡 Minor Issues

### JT-001: Rust Milli.sub() loses sign information

**File:** `rust/jsontime/src/milli.rs:54-57`

**Description:**  
The `sub()` method returns an unsigned `Duration`, losing the sign when the result would be negative:

```rust
pub fn sub(&self, other: &Self) -> Duration {
    let diff = self.0.signed_duration_since(other.0);
    Duration::from_millis(diff.num_milliseconds().unsigned_abs())
}
```

**Impact:** Cannot determine if `self` is before or after `other` from the result alone.

**Workaround:** Use `before()` or `after()` methods to check ordering first.

---

### JT-002: Rust Unix.sub() same issue

**File:** `rust/jsontime/src/unix.rs:54-57`

**Description:**  
Same issue as JT-001 - loses sign information.

---

### JT-003: Rust Duration parsing more restrictive than Go

**File:** `rust/jsontime/src/duration.rs:123-158`

**Description:**  
Go's `time.ParseDuration` supports more units:
- `ns` (nanoseconds)
- `us`/`µs` (microseconds)
- `ms` (milliseconds)

Rust implementation only supports `h`, `m`, `s`.

**Impact:** Duration strings with sub-second units fail to parse in Rust.

**Example:**
```go
// Go - works
d, _ := time.ParseDuration("100ms")

// Rust - fails
let d: Duration = serde_json::from_str(r#""100ms""#)?;  // Error!
```

---

### JT-004: Rust Duration cannot be negative

**Description:**  
Go's `time.Duration` is signed (int64), Rust's `std::time::Duration` is unsigned.

**Impact:** Cannot represent negative durations in Rust.

**Status:** By design (Rust stdlib limitation).

---

## 🔵 Enhancements

### JT-005: Missing microsecond timestamp type

**Description:**  
Some APIs (particularly high-frequency systems) use microsecond timestamps. Neither Go nor Rust implementation provides a `Micro` type.

**Suggestion:** Add `Micro` type for microsecond precision.

---

### JT-006: Missing nanosecond timestamp type

**Description:**  
Some APIs use nanosecond timestamps. No `Nano` type provided.

**Suggestion:** Add `Nano` type for nanosecond precision.

---

### JT-007: Go Duration lacks explicit constructors

**Description:**  
Go implementation lacks explicit constructors like Rust has:
- `from_secs()`
- `from_millis()`

**Current Go usage:**
```go
d := Duration(30 * time.Second)
```

**Suggested addition:**
```go
func DurationFromSeconds(s int64) Duration
func DurationFromMillis(ms int64) Duration
```

---

## ⚪ Notes

### JT-008: Different underlying time libraries

**Description:**  
- Go: Uses stdlib `time.Time`
- Rust: Uses `chrono::DateTime<Utc>`

**Impact:** Rust has hard dependency on `chrono` crate.

---

### JT-009: Rust types implement more traits

**Description:**  
Rust types implement `PartialOrd`, `Ord`, `Hash` which enables use in collections:

```rust
use std::collections::HashSet;
let mut times: HashSet<Unix> = HashSet::new();
times.insert(Unix::now());
```

Go types don't have equivalent functionality.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| JT-001 | 🟡 Minor | Open | Rust Milli |
| JT-002 | 🟡 Minor | Open | Rust Unix |
| JT-003 | 🟡 Minor | Open | Rust Duration |
| JT-004 | 🟡 Minor | By design | Rust Duration |
| JT-005 | 🔵 Enhancement | Open | Both |
| JT-006 | 🔵 Enhancement | Open | Both |
| JT-007 | 🔵 Enhancement | Open | Go |
| JT-008 | ⚪ Note | N/A | Rust |
| JT-009 | ⚪ Note | N/A | Rust |

**Overall:** Functional implementation. Main concern is duration parsing parity between Go and Rust.
