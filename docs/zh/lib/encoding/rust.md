# Encoding Package - Rust Implementation

Crate: `giztoy-encoding`

📚 [Rust Documentation](https://docs.rs/giztoy-encoding)

## Types

### StdBase64Data

```rust
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default)]
pub struct StdBase64Data(Vec<u8>);
```

A newtype wrapper around `Vec<u8>` that serializes to/from standard Base64.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(data: Vec<u8>) -> Self` | Create from Vec |
| `empty` | `fn empty() -> Self` | Create empty |
| `as_bytes` | `fn as_bytes(&self) -> &[u8]` | Get byte slice reference |
| `as_bytes_mut` | `fn as_bytes_mut(&mut self) -> &mut Vec<u8>` | Get mutable reference |
| `into_bytes` | `fn into_bytes(self) -> Vec<u8>` | Consume and return Vec |
| `is_empty` | `fn is_empty(&self) -> bool` | Check if empty |
| `len` | `fn len(&self) -> usize` | Get length |
| `encode` | `fn encode(&self) -> String` | Encode to Base64 string |
| `decode` | `fn decode(s: &str) -> Result<Self, DecodeError>` | Decode from Base64 |

**Trait Implementations:**
- `Serialize` / `Deserialize` (serde)
- `Display` (formats as Base64)
- `Deref<Target=[u8]>` / `DerefMut`
- `From<Vec<u8>>`, `From<&[u8]>`, `From<[u8; N]>`
- `AsRef<[u8]>`

### HexData

```rust
#[derive(Debug, Clone, PartialEq, Eq, Hash, Default)]
pub struct HexData(Vec<u8>);
```

A newtype wrapper around `Vec<u8>` that serializes to/from hexadecimal.

**Methods:**

Same API as `StdBase64Data`, but with hex encoding:

| Method | Signature | Description |
|--------|-----------|-------------|
| `encode` | `fn encode(&self) -> String` | Encode to hex string |
| `decode` | `fn decode(s: &str) -> Result<Self, FromHexError>` | Decode from hex |

## Usage

### In Struct Fields

```rust
use giztoy_encoding::{StdBase64Data, HexData};
use serde::{Serialize, Deserialize};

#[derive(Serialize, Deserialize)]
struct Message {
    id: String,
    payload: StdBase64Data,
    hash: HexData,
}

let msg = Message {
    id: "msg-123".to_string(),
    payload: StdBase64Data::from(b"hello world".as_slice()),
    hash: HexData::from(vec![0xab, 0xcd, 0xef]),
};

// Serializes to:
// {"id":"msg-123","payload":"aGVsbG8gd29ybGQ=","hash":"abcdef"}
let json = serde_json::to_string(&msg).unwrap();
```

### Standalone Encoding

```rust
// Base64
let data = StdBase64Data::from(b"hello".as_slice());
println!("{}", data);  // "aGVsbG8="
println!("{}", data.encode());  // "aGVsbG8="

// Hex
let hash = HexData::from(vec![0xde, 0xad]);
println!("{}", hash);  // "dead"
```

### Deref Coercion

```rust
let data = StdBase64Data::from(vec![1, 2, 3]);

// Can use as &[u8] directly
fn process(bytes: &[u8]) { /* ... */ }
process(&data);  // Deref coercion

// Access slice methods
println!("len: {}", data.len());
println!("first: {:?}", data.first());
```

### Null Handling

```rust
// Null deserializes to empty
let data: StdBase64Data = serde_json::from_str("null").unwrap();
assert!(data.is_empty());

// Empty string also empty
let data: StdBase64Data = serde_json::from_str(r#""""#).unwrap();
assert!(data.is_empty());
```

## Dependencies

- `base64` crate (for Base64 encoding)
- `hex` crate (for hex encoding)
- `serde` crate (for serialization)

## Differences from Go

| Aspect | Go | Rust |
|--------|----|----- |
| Type structure | Type alias `[]byte` | Newtype `struct(Vec<u8>)` |
| Conversion to bytes | Direct cast | `.as_bytes()` or `Deref` |
| Additional methods | None | `is_empty()`, `len()`, `encode()`, `decode()` |
| Hash/Eq traits | N/A (slice) | Implemented |
| Clone | Implicit | Explicit (implemented) |
