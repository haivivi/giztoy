# RegisterFunc Implementation Tracking

> **Status**: ✅ Complete  
> **Started**: 2026-01-26  
> **Last Updated**: 2026-01-26

---

## Overview

Implement `RegisterFunc` to allow Go/Rust functions to be called from Luau scripts.

### Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Luau VM    │ ──> │  C Wrapper  │ ──> │  Go/Rust    │
│  calls fn   │     │  callback   │     │  lookup &   │
│             │     │  with ID    │     │  call func  │
└─────────────┘     └─────────────┘     └─────────────┘
```

---

## Implementation Checklist

### Phase 1: C Wrapper Layer ✅

- [x] Add `LuauExternalCallback` typedef
- [x] Add `luau_setexternalcallback` API
- [x] Add `luau_pushexternalfunc` API with callback_id upvalues
- [x] Add `luau_registerexternal` API
- [x] Add `luau_getcallbackid` helper

### Phase 2: Go Bindings ✅

- [x] Create global function registry with mutex
- [x] Export `goExternalCallback` function via CGO
- [x] Implement `RegisterFunc(name string, fn GoFunc)`
- [x] Implement `UnregisterFunc(name string)`
- [x] Handle State cleanup (remove registered funcs on Close)
- [x] Unit tests for basic functionality

### Phase 3: Rust Bindings ✅

- [x] Create global function registry with RwLock
- [x] Export `rust_external_callback` function via FFI
- [x] Implement `register_func(name: &str, fn: impl Fn)`
- [x] Implement `unregister_func(name: &str)`
- [x] Handle State cleanup with Drop
- [x] Unit tests for basic functionality

### Phase 4: Functional Tests ✅

| Test | Go | Rust |
|------|:--:|:----:|
| `test_register_simple` | [x] | [x] |
| `test_register_with_args` | [x] | [x] |
| `test_register_multi_return` | [x] | [x] |
| `test_register_table_arg` | [x] | [x] |
| `test_register_overwrite` | [x] | [x] |
| `test_nested_call` | [x] | [x] |
| `test_error_handling` | [x] | - |
| `test_nil_callback` | [x] | - |

### Phase 5: Memory Management Tests ✅

| Test | Go | Rust |
|------|:--:|:----:|
| `test_memory_register_many` | [x] | [x] |
| `test_memory_call_loop` | [x] | [x] |
| `test_memory_string_args` | [x] | [x] |
| `test_memory_state_close` | [x] | [x] |
| `test_memory_gc_interaction` | [x] | [x] |

### Phase 6: Concurrency Tests ✅

| Test | Go | Rust |
|------|:--:|:----:|
| `test_concurrent_register_multi_state` | [x] | [x] |
| `test_concurrent_call_different_states` | [x] | [x] |
| `test_concurrent_state_creation_destruction` | [x] | [x] |
| `test_concurrent_registry_access` | [x] | [x] |

**Note**: Luau State is NOT thread-safe. Concurrent tests use separate States per thread.

### Phase 7: Benchmarks ✅

| Benchmark | Go | Rust |
|-----------|:--:|:----:|
| `bench_register_func` | [x] | [x] |
| `bench_call_no_args` | [x] | [x] |
| `bench_call_with_args` | [x] | [x] |
| `bench_call_return_string` | [x] | [x] |
| `bench_register_1000_funcs` | [x] | [x] |
| `bench_memory_pressure` | [x] | [x] |

### Phase 8: Edge Cases ✅

| Test | Go | Rust |
|------|:--:|:----:|
| `test_empty_name` | [x] | [x] |
| `test_special_chars_name` | [x] | [x] |
| `test_unicode_name` | [x] | [x] |
| `test_call_unregistered` | [x] | [x] |
| `test_deep_recursion` | [x] | [x] |

---

## Key Verification Points

### Memory Safety ✅
- [x] No memory leaks (verified via tests)
- [x] No use-after-free
- [x] `State.Close()` / `Drop` correctly cleans up global registry

### Thread Safety ✅
- [x] Global function registry properly locked (mutex/RwLock)
- [x] Different States are isolated from each other
- [x] Concurrent tests pass

### Error Handling ✅
- [x] Go panic recovery in callback
- [x] Rust `catch_unwind` in callback
- [x] Nil/invalid callback detection

---

## Progress Log

### 2026-01-26
- [x] Created tracking document
- [x] Phase 1: C Wrapper Layer - completed
  - Added `LuauExternalCallback` typedef
  - Added `luau_setexternalcallback`, `luau_pushexternalfunc`, `luau_registerexternal`, `luau_getcallbackid`
- [x] Phase 2: Go Bindings - completed
  - Global function registry with mutex
  - `RegisterFunc` and `UnregisterFunc` implementation
  - Cleanup on State.Close()
- [x] Phase 3: Rust Bindings - completed
  - Global function registry with RwLock
  - `register_func` and `unregister_func` implementation
  - Cleanup on Drop
- [x] Phase 4: Functional Tests - completed
  - 8+ tests in Go, 8+ tests in Rust
- [x] Phase 5: Memory Management Tests - completed
  - 5 tests each for Go and Rust
- [x] Phase 6: Concurrency Tests - completed
  - 4 tests each for Go and Rust
  - Note: Luau State is not thread-safe, tests use separate States
- [x] Phase 7: Benchmarks - completed
  - 6 benchmarks each for Go and Rust
- [x] Phase 8: Edge Cases - completed
  - 5 edge case tests each

---

## Notes

### Design Decisions

1. **Global Registry vs Per-State Registry**
   - Decision: Global registry with per-State cleanup
   - Rationale: Simpler implementation, allows callbacks to access State

2. **Function ID Type**
   - Decision: `uint64_t` stored as two 32-bit integers in upvalues
   - Rationale: 32-bit compatibility for Luau VM

3. **Error Propagation**
   - Decision: Panic recovery returns 0 (no values), no error push
   - Rationale: Prevents crashes and stack pollution. Pushing an error string
     but returning 0 would leave orphaned values on the Luau stack.

4. **Thread Safety**
   - Decision: Global mutex (Go) / RwLock (Rust) for registry
   - Rationale: Required for concurrent State creation/destruction
   - Note: Individual State operations are NOT thread-safe

### Known Limitations

1. **Luau State is not thread-safe** - Each goroutine/thread must use its own State
2. **Unicode identifiers** - Luau doesn't support Unicode in identifiers, but can access via `_G["name"]`
3. **Panic in callback** - Panics are caught but may leave Luau in inconsistent state

### Files Modified

**C Wrapper:**
- `luau/c/luau_wrapper.h` - Added external callback types and functions
- `luau/c/luau_wrapper.cpp` - Implemented external callback support

**Go:**
- `go/pkg/luau/luau.go` - Added RegisterFunc, global registry
- `go/pkg/luau/luau_test.go` - Added 30+ tests and 6 benchmarks

**Rust:**
- `rust/luau/src/lib.rs` - Added register_func, global registry
- `rust/luau/src/ffi.rs` - Added FFI declarations
- `rust/luau/src/tests.rs` - Added 30+ tests
- `rust/luau/benches/luau_bench.rs` - Added 6 benchmarks
- `rust/luau/BUILD.bazel` - Added benchmark target
