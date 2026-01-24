# Buffer Package - Known Issues

## 🟠 Major Issues

### BUF-001: Go Buffer.Next() uses LIFO instead of FIFO

**File:** `go/pkg/buffer/buffer.go:259-262`

**Description:**  
The `Next()` method reads from the END of the buffer (LIFO behavior), while `Read()` reads from the front (FIFO). This inconsistency is confusing and likely unintentional.

```go
// Current implementation (LIFO)
head := len(b.buf) - 1
t = b.buf[head]
b.buf = b.buf[:head]
```

**Expected:** Should read from `b.buf[0]` for FIFO consistency with `Read()`.

**Impact:** Users expecting iterator-style sequential access get reversed order.

**Status:** ⚠️ Documented in code comment but should be fixed.

---

### BUF-002: Go Buffer.Add() missing write notification

**File:** `go/pkg/buffer/buffer.go:280-291`

**Description:**  
The `Add()` method appends an element but does NOT send a notification on `writeNotify`. If a reader is blocked waiting and only `Add()` is used for writing, the reader may block indefinitely.

```go
func (b *Buffer[T]) Add(t T) error {
    // ... error checks ...
    b.buf = append(b.buf, t)
    return nil  // Missing: select { case b.writeNotify <- struct{}{}: default: }
}
```

**Impact:** Potential deadlock when using `Add()` exclusively.

**Status:** 🔴 Bug - needs fix.

---

## 🟡 Minor Issues

### BUF-003: Go Buffer.Bytes() returns internal slice reference

**File:** `go/pkg/buffer/buffer.go:335-339`

**Description:**  
`Bytes()` returns the internal slice directly, not a copy. Modifications to the returned slice will corrupt the buffer state.

```go
func (b *Buffer[T]) Bytes() []T {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.buf  // Returns internal reference!
}
```

**Impact:** Data corruption if caller modifies the returned slice.

**Workaround:** Document clearly or change to return a copy.

---

### BUF-004: Go BlockBuffer.Bytes() inconsistent copy behavior

**File:** `go/pkg/buffer/block_buffer.go:356-365`

**Description:**  
Documentation says "returned slice is a copy" but when `h < t`, it returns a subslice of the internal buffer directly:

```go
if h < t {
    return bb.buf[h:t]  // Not a copy!
}
return slices.Concat(bb.buf[h:], bb.buf[:t])  // This is a copy
```

**Impact:** Inconsistent behavior depending on buffer state.

---

### BUF-005: Go RingBuffer.Bytes() same issue as BUF-004

**File:** `go/pkg/buffer/ring_buffer.go:306-315`

**Description:**  
Same inconsistent copy behavior as BlockBuffer.

---

## 🔵 Enhancements

### BUF-006: Rust BlockBuffer uses Vec<Option<T>> overhead

**File:** `rust/buffer/src/block_buffer.rs:82`

**Description:**  
Rust implementation uses `Vec<Option<T>>` which adds memory overhead (size of discriminant per element) compared to Go's direct slice approach.

**Suggestion:** Consider using `MaybeUninit<T>` with careful initialization tracking for zero-cost abstraction.

---

### BUF-007: Go/Rust Buffer.Next() semantic difference

**Description:**  
- Go: `Next()` is LIFO (pops from end)
- Rust: `next()` is FIFO (pops from front via VecDeque)

This API inconsistency could cause bugs when porting code between languages.

**Suggestion:** Align Go implementation to match Rust (FIFO).

---

## ⚪ Notes

### BUF-008: No io.Reader/io.Writer implementation in Rust

**Description:**  
Go buffers implement `io.Reader` and `io.Writer` interfaces. Rust buffers don't implement `std::io::Read` and `std::io::Write` traits.

**Reason:** Rust buffers are generic over `T: Clone`, not just bytes.

**Suggestion:** Add byte-specific wrapper types that implement std::io traits.

---

### BUF-009: Missing Bytes() equivalent in Go BytesBuffer interface

**File:** `go/pkg/buffer/bytes.go:12-23`

**Description:**  
The `BytesBuffer` interface includes `Bytes() []byte` but this is dangerous given BUF-003/004/005.

**Suggestion:** Consider removing from interface or ensuring all implementations return copies.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| BUF-001 | 🟠 Major | Open | Go Buffer |
| BUF-002 | 🟠 Major | Open | Go Buffer |
| BUF-003 | 🟡 Minor | Open | Go Buffer |
| BUF-004 | 🟡 Minor | Open | Go BlockBuffer |
| BUF-005 | 🟡 Minor | Open | Go RingBuffer |
| BUF-006 | 🔵 Enhancement | Open | Rust BlockBuffer |
| BUF-007 | 🔵 Enhancement | Open | Go/Rust parity |
| BUF-008 | ⚪ Note | N/A | Rust |
| BUF-009 | ⚪ Note | N/A | Go Interface |
