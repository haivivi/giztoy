# Trie Package - Known Issues

## 🟡 Minor Issues

### TRI-001: Go Walk visits unset nodes

**File:** `go/pkg/trie/trie.go:175-179`

**Description:**  
The `Walk` function visits ALL nodes including those without values set, passing the zero value:

```go
func (t *Trie[T]) Walk(f func(path string, value T, set bool)) {
    t.walkWithPath(nil, func(path []string, node *Trie[T]) {
        f(strings.Join(path, "/"), node.value, node.set)  // value may be zero
    })
}
```

**Impact:** Callers must check the `set` boolean to filter actual values.

**Suggestion:** Consider only visiting nodes where `set == true` by default.

---

### TRI-002: Go Len() is O(n) not O(1)

**File:** `go/pkg/trie/trie.go:211-219`

**Description:**  
`Len()` walks the entire trie to count values:

```go
func (t *Trie[T]) Len() int {
    count := 0
    t.Walk(func(_ string, _ T, set bool) {
        if set {
            count++
        }
    })
    return count
}
```

**Impact:** Performance issue for large tries with frequent `Len()` calls.

**Suggestion:** Maintain a counter that increments on Set and decrements on Delete.

---

### TRI-003: Rust Len() same O(n) issue

**File:** `rust/trie/src/lib.rs:292-296`

**Description:**  
Same issue as Go - walks entire trie to count.

---

### TRI-004: No Delete operation

**Description:**  
Neither Go nor Rust implementation provides a way to delete/remove values from the trie.

**Impact:** Cannot remove stale subscriptions or routes without rebuilding.

**Suggestion:** Add `Delete(path string) bool` method.

---

### TRI-005: Go Match returns route with leading slash inconsistency

**File:** `go/pkg/trie/trie.go:142-172`

**Description:**  
When building the matched route string, it prepends "/" to each segment:

```go
ch.match(matched+"/"+first, subseq)  // Results in "/device/+/state"
```

But the root path returns empty string, creating inconsistency.

---

## 🔵 Enhancements

### TRI-006: No thread safety

**Description:**  
Neither implementation is thread-safe. Concurrent read/write will cause data races.

**Go:**
```go
// UNSAFE: concurrent access
go trie.Set("a/b", value1)
go trie.Set("a/c", value2)
```

**Suggestion:** Add `sync.RWMutex` wrapper or document thread-safety requirements.

---

### TRI-007: No path parameter extraction

**Description:**  
When matching `device/+/state` against `device/gear-001/state`, there's no way to extract `gear-001` as a parameter.

**Current:** Only returns the matched route pattern and value.

**Suggestion:** Add `MatchParams(path) (params map[string]string, value *T, ok bool)`.

---

### TRI-008: No prefix listing

**Description:**  
Cannot list all paths under a prefix efficiently.

**Example use case:** List all devices under `device/` prefix.

**Suggestion:** Add `List(prefix string) []string` method.

---

## ⚪ Notes

### TRI-009: Different value storage approaches

**Description:**  
- Go: Uses `set bool` flag with zero value
- Rust: Uses `Option<T>`

Both approaches work but have different trade-offs:
- Go: Can distinguish "set to zero" vs "not set"
- Rust: More idiomatic, less memory overhead

---

### TRI-010: Path leading slash handling

**Description:**  
Paths can start with or without `/`:
- `"/a/b/c"` and `"a/b/c"` are NOT equivalent
- Leading `/` creates an empty string segment

```go
trie.SetValue("/a/b", "val1")  // path segments: ["", "a", "b"]
trie.SetValue("a/b", "val2")   // path segments: ["a", "b"]
```

**Status:** Documented behavior but may be confusing.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| TRI-001 | 🟡 Minor | Open | Go Walk |
| TRI-002 | 🟡 Minor | Open | Go Len |
| TRI-003 | 🟡 Minor | Open | Rust Len |
| TRI-004 | 🟡 Minor | Open | Both |
| TRI-005 | 🟡 Minor | Open | Go Match |
| TRI-006 | 🔵 Enhancement | Open | Both |
| TRI-007 | 🔵 Enhancement | Open | Both |
| TRI-008 | 🔵 Enhancement | Open | Both |
| TRI-009 | ⚪ Note | N/A | Both |
| TRI-010 | ⚪ Note | N/A | Both |

**Overall:** Solid implementation for basic trie operations. Missing Delete and thread-safety for production use.
