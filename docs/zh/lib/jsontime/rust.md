# JsonTime Package - Rust Implementation

Crate: `giztoy-jsontime`

📚 [Rust Documentation](https://docs.rs/giztoy-jsontime)

## Types

### Unix

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Unix(DateTime<Utc>);
```

A timestamp that serializes to/from Unix seconds in JSON.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(dt: DateTime<Utc>) -> Self` | Create from DateTime |
| `now` | `fn now() -> Self` | Current time |
| `from_secs` | `fn from_secs(secs: i64) -> Self` | Create from seconds |
| `as_secs` | `fn as_secs(&self) -> i64` | Get seconds value |
| `datetime` | `fn datetime(&self) -> DateTime<Utc>` | Get underlying DateTime |
| `before` | `fn before(&self, other: &Self) -> bool` | Is this before other? |
| `after` | `fn after(&self, other: &Self) -> bool` | Is this after other? |
| `is_zero` | `fn is_zero(&self) -> bool` | Is zero time? |
| `sub` | `fn sub(&self, other: &Self) -> Duration` | Duration between times |
| `add` | `fn add(&self, d: Duration) -> Self` | Return self+d |

**Trait Implementations:**
- `Serialize` / `Deserialize` (serde)
- `Display`
- `From<DateTime<Utc>>`, `From<i64>`
- `PartialOrd`, `Ord`, `Hash`

### Milli

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Milli(DateTime<Utc>);
```

A timestamp that serializes to/from Unix milliseconds in JSON.

**Methods:** Same as Unix, but with milliseconds.

| Method | Signature | Description |
|--------|-----------|-------------|
| `from_millis` | `fn from_millis(ms: i64) -> Self` | Create from milliseconds |
| `as_millis` | `fn as_millis(&self) -> i64` | Get milliseconds value |
| ... | | (same operations as Unix) |

### Duration

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Duration(StdDuration);
```

A duration that serializes to string (e.g., "1h30m") and deserializes from string or nanoseconds.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(d: StdDuration) -> Self` | Create from std Duration |
| `from_secs` | `fn from_secs(secs: u64) -> Self` | Create from seconds |
| `from_millis` | `fn from_millis(ms: u64) -> Self` | Create from milliseconds |
| `from_nanos` | `fn from_nanos(nanos: u64) -> Self` | Create from nanoseconds |
| `as_std` | `fn as_std(&self) -> StdDuration` | Get std Duration |
| `as_secs` | `fn as_secs(&self) -> u64` | Get whole seconds |
| `as_secs_f64` | `fn as_secs_f64(&self) -> f64` | Get floating seconds |
| `as_millis` | `fn as_millis(&self) -> u128` | Get milliseconds |
| `as_nanos` | `fn as_nanos(&self) -> u128` | Get nanoseconds |
| `is_zero` | `fn is_zero(&self) -> bool` | Is zero duration? |

## Usage

### In Struct Fields

```rust
use giztoy_jsontime::{Unix, Milli, Duration};
use serde::{Serialize, Deserialize};

#[derive(Serialize, Deserialize)]
struct Event {
    id: String,
    created_at: Unix,
    expires_at: Milli,
    timeout: Duration,
}

let event = Event {
    id: "evt-123".to_string(),
    created_at: Unix::now(),
    expires_at: Milli::now(),
    timeout: Duration::from_secs(30),
};

// Serializes to:
// {"id":"evt-123","created_at":1705315800,"expires_at":1705315800000,"timeout":"30s"}
```

### Duration Parsing

```rust
use giztoy_jsontime::Duration;

// String format
let d: Duration = serde_json::from_str(r#""1h30m""#).unwrap();
assert_eq!(d.as_secs(), 5400);

// Integer format (nanoseconds)
let d: Duration = serde_json::from_str("5400000000000").unwrap();
assert_eq!(d.as_secs(), 5400);
```

### Time Arithmetic

```rust
use giztoy_jsontime::Unix;
use std::time::Duration;

let now = Unix::now();
let later = now.add(Duration::from_secs(86400));

if later.after(&now) {
    let diff = later.sub(&now);
    println!("{:?}", diff);  // 86400s
}
```

### From Conversions

```rust
// From i64
let unix = Unix::from(1705315800i64);
let milli = Milli::from(1705315800000i64);

// From DateTime<Utc>
let unix = Unix::from(Utc::now());

// From std::time::Duration
let dur = Duration::from(std::time::Duration::from_secs(60));
```

## Duration String Format

The parser supports Go-style duration strings:

| Input | Parsed As |
|-------|-----------|
| `"1h"` | 3600 seconds |
| `"30m"` | 1800 seconds |
| `"45s"` | 45 seconds |
| `"1h30m"` | 5400 seconds |
| `"1h30m45s"` | 5445 seconds |
| `""` | 0 seconds |

## Dependencies

- `chrono` crate (for DateTime handling)
- `serde` crate (for serialization)

## Differences from Go

| Aspect | Go | Rust |
|--------|----|----- |
| Time type | Type alias `time.Time` | Newtype over `DateTime<Utc>` |
| Duration range | Signed (int64 ns) | Unsigned (u64 + u32 ns) |
| Ordering | Via method calls | Via `Ord` trait |
| Hash support | N/A | Implemented |
| sub() return | Signed duration | Unsigned duration |
