# Trie Package

Generic trie data structure for efficient path-based storage and retrieval with MQTT-style wildcard support.

## Design Goals

1. **Efficient Path Matching**: O(k) lookup where k is path depth
2. **Wildcard Support**: MQTT-style single (`+`) and multi-level (`#`) wildcards
3. **Generic Storage**: Store any value type at path nodes
4. **Zero-Copy Lookups**: Minimize allocations during get operations

## Wildcard Patterns

The trie supports MQTT-style topic patterns:

| Pattern | Description | Example Match |
|---------|-------------|---------------|
| `a/b/c` | Exact path | `a/b/c` only |
| `a/+/c` | Single-level wildcard | `a/X/c`, `a/Y/c` |
| `a/#` | Multi-level wildcard | `a/b`, `a/b/c/d` |

### Pattern Rules

1. **`+` (Plus)**: Matches exactly one path segment
   - `device/+/state` matches `device/gear-001/state`
   - Does NOT match `device/gear-001/sub/state`

2. **`#` (Hash)**: Matches zero or more path segments
   - Must be the last segment in the pattern
   - `logs/#` matches `logs`, `logs/app`, `logs/app/debug/line1`

3. **Priority**: Exact matches take precedence over wildcards
   - If both `device/gear-001/state` and `device/+/state` exist, exact wins

## Use Cases

### MQTT Topic Routing

```
device/+/state     -> state_handler
device/+/command   -> command_handler
logs/#             -> log_handler
```

### API Path Routing

```
/users/{id}/profile  -> profile_handler
/users/{id}/posts    -> posts_handler
/admin/#             -> admin_handler
```

### Hierarchical Configuration

```
app/database/host    -> "localhost"
app/database/port    -> 5432
app/cache/#          -> cache_config
```

## Performance Characteristics

| Operation | Complexity | Notes |
|-----------|------------|-------|
| Set | O(k) | k = path depth |
| Get | O(k) | Zero allocation in Rust |
| Walk | O(n) | n = total nodes |
| Len | O(n) | Counts all values |

## Examples Directory

- `examples/go/trie/` - Go usage examples (if any)
- `examples/rust/trie/` - Rust usage examples (if any)

## Related Packages

- `mqtt0` - Uses trie for topic subscription matching
- `chatgear` - Uses trie for message routing
