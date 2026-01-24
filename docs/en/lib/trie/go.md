# Trie Package - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/trie`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/trie)

## Types

### Trie[T]

```go
type Trie[T any] struct {
    children map[string]*Trie[T] // exact path segment matches
    matchAny *Trie[T]            // single-level wildcard (+)
    matchAll *Trie[T]            // multi-level wildcard (#)
    set      bool                // whether this node has a value
    value    T                   // the value stored
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `New` | `func New[T any]() *Trie[T]` | Create empty trie |
| `Set` | `(t *Trie[T]) Set(path string, setFunc func(*T, bool) error) error` | Set with custom setter |
| `SetValue` | `(t *Trie[T]) SetValue(path string, value T) error` | Set value directly |
| `Get` | `(t *Trie[T]) Get(path string) (*T, bool)` | Get value pointer |
| `GetValue` | `(t *Trie[T]) GetValue(path string) (T, bool)` | Get value copy |
| `Match` | `(t *Trie[T]) Match(path string) (route string, value *T, ok bool)` | Get with matched route |
| `Walk` | `(t *Trie[T]) Walk(f func(path string, value T, set bool))` | Visit all nodes |
| `Len` | `(t *Trie[T]) Len() int` | Count values |
| `String` | `(t *Trie[T]) String() string` | Debug representation |

### ErrInvalidPattern

```go
var ErrInvalidPattern = errors.New("invalid path pattern...")
```

Returned when `#` wildcard is not at the end of the path.

## Usage

### Basic Set/Get

```go
tr := trie.New[string]()

// Set exact path
tr.SetValue("device/gear-001/state", "online")

// Get value
val, ok := tr.GetValue("device/gear-001/state")
// val = "online", ok = true
```

### Wildcard Patterns

```go
tr := trie.New[string]()

// Single-level wildcard
tr.SetValue("device/+/state", "state_handler")

// Multi-level wildcard
tr.SetValue("logs/#", "log_handler")

// Match against patterns
val, _ := tr.GetValue("device/any-device/state")
// val = "state_handler"

val, _ = tr.GetValue("logs/app/debug/line1")
// val = "log_handler"
```

### Custom Set Function

```go
tr := trie.New[[]string]()

// Append to existing value
tr.Set("handlers/events", func(ptr *[]string, existed bool) error {
    if !existed {
        *ptr = []string{"handler1"}
    } else {
        *ptr = append(*ptr, "handler2")
    }
    return nil
})
```

### Walk All Nodes

```go
tr.Walk(func(path string, value string, set bool) {
    if set {
        fmt.Printf("%s: %s\n", path, value)
    }
})
```

### Match with Route

```go
tr := trie.New[string]()
tr.SetValue("device/+/state", "handler")

route, value, ok := tr.Match("device/gear-001/state")
// route = "/+/state"
// value = "handler"
// ok = true
```

## Implementation Details

### Path Splitting

Paths are split by `/` and processed segment by segment:

```go
// "device/gear-001/state" splits into:
// first="device", subseq="gear-001/state"
```

### Value Storage

Uses a `set` boolean flag to distinguish between:
- Value not set (default zero value)
- Value explicitly set to zero value

### Match Priority

1. Exact child match
2. Single-level wildcard (`+`)
3. Multi-level wildcard (`#`)

## Benchmarks

Typical performance (from benchmarks):

| Operation | 100 paths | 1000 paths | 10000 paths |
|-----------|-----------|------------|-------------|
| Set all | ~50µs | ~500µs | ~5ms |
| Get (exact) | ~10µs | ~100µs | ~1ms |
| Walk | ~5µs | ~50µs | ~500µs |
