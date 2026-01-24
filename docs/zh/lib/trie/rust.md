# Trie Package - Rust Implementation

Crate: `giztoy-trie`

📚 [Rust Documentation](https://docs.rs/giztoy-trie)

## Types

### Trie<T>

```rust
#[derive(Debug, Clone)]
pub struct Trie<T> {
    children: HashMap<String, Trie<T>>,
    match_any: Option<Box<Trie<T>>>,  // single-level wildcard (+)
    match_all: Option<Box<Trie<T>>>,  // multi-level wildcard (#)
    value: Option<T>,
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new() -> Self` | Create empty trie |
| `set` | `fn set<F, E>(&mut self, path: &str, setter: F) -> Result<(), E>` | Set with custom setter |
| `set_value` | `fn set_value(&mut self, path: &str, value: T) -> Result<(), InvalidPatternError>` | Set value directly |
| `get` | `fn get(&self, path: &str) -> Option<&T>` | Get value reference (zero-alloc) |
| `get_value` | `fn get_value(&self, path: &str) -> Option<T>` | Get cloned value |
| `match_path` | `fn match_path(&self, path: &str) -> (String, Option<&T>)` | Get with matched route |
| `walk` | `fn walk<F>(&self, f: F)` | Visit all nodes |
| `len` | `fn len(&self) -> usize` | Count values |
| `is_empty` | `fn is_empty(&self) -> bool` | Check if empty |

**Trait Implementations:**
- `Default`
- `Clone`
- `Debug`
- `Display` (when `T: Display`)

### InvalidPatternError

```rust
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("invalid path pattern: path should be /a/b/c or /a/+/c or /a/#")]
pub struct InvalidPatternError;
```

## Usage

### Basic Set/Get

```rust
use giztoy_trie::Trie;

let mut trie = Trie::<String>::new();

// Set exact path
trie.set_value("device/gear-001/state", "online".to_string()).unwrap();

// Get value (zero allocation)
let val: Option<&String> = trie.get("device/gear-001/state");

// Get cloned value
let val: Option<String> = trie.get_value("device/gear-001/state");
```

### Wildcard Patterns

```rust
let mut trie = Trie::<String>::new();

// Single-level wildcard
trie.set_value("device/+/state", "state_handler".to_string()).unwrap();

// Multi-level wildcard
trie.set_value("logs/#", "log_handler".to_string()).unwrap();

// Match against patterns
let val = trie.get("device/any-device/state");
assert_eq!(val, Some(&"state_handler".to_string()));

let val = trie.get("logs/app/debug/line1");
assert_eq!(val, Some(&"log_handler".to_string()));
```

### Custom Set Function

```rust
let mut trie = Trie::<Vec<String>>::new();

// Set with custom logic
trie.set("handlers/events", |existing| {
    match existing {
        Some(vec) => {
            vec.push("handler2".to_string());
            Ok(vec.clone())
        }
        None => Ok(vec!["handler1".to_string()]),
    }
}).unwrap();
```

### Walk All Nodes

```rust
trie.walk(|path, value| {
    println!("{}: {}", path, value);
});
```

### Match with Route

```rust
let mut trie = Trie::<String>::new();
trie.set_value("device/+/state", "handler".to_string()).unwrap();

let (route, value) = trie.match_path("device/gear-001/state");
// route = "/device/+/state"
// value = Some(&"handler")
```

## Implementation Details

### Zero-Allocation Lookup

The `get()` method performs zero allocations by:
- Using string slices for path splitting
- Returning references instead of cloned values

```rust
#[inline]
fn split_path(path: &str) -> (&str, &str) {
    match path.find('/') {
        Some(idx) => (&path[..idx], &path[idx + 1..]),
        None => (path, ""),
    }
}
```

### Value Storage

Uses `Option<T>` instead of a separate flag:
- `None` = value not set
- `Some(T)` = value set

### Wildcard Storage

- `match_any`: `Option<Box<Trie<T>>>` for `+` wildcard
- `match_all`: `Option<Box<Trie<T>>>` for `#` wildcard

Boxed to avoid recursive type sizing issues.

## Differences from Go

| Aspect | Go | Rust |
|--------|----|----- |
| Value storage | `set bool` + `value T` | `Option<T>` |
| Child storage | `map[string]*Trie[T]` | `HashMap<String, Trie<T>>` |
| Wildcard storage | `*Trie[T]` (pointer) | `Option<Box<Trie<T>>>` |
| Get return | `(*T, bool)` | `Option<&T>` |
| Clone support | Implicit (pointer) | Explicit `Clone` derive |
| Zero-alloc get | No (returns route string) | Yes (`get()` method) |
