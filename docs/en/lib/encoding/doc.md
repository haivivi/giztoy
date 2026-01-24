# Encoding Package

JSON-serializable encoding types for binary data.

## Design Goals

1. **Seamless JSON Integration**: Binary data that automatically serializes to human-readable formats
2. **Type Safety**: Distinct types for different encodings prevent mixing
3. **Zero-Copy Where Possible**: Minimal allocations during serialization

## Types

| Type | Encoding | JSON Example | Use Case |
|------|----------|--------------|----------|
| `StdBase64Data` | Standard Base64 | `"aGVsbG8="` | Binary payloads, files |
| `HexData` | Hexadecimal | `"deadbeef"` | Hashes, IDs, debugging |

## Features

### JSON Serialization

Both types implement JSON marshal/unmarshal:

```json
{
  "payload": "aGVsbG8gd29ybGQ=",
  "hash": "a1b2c3d4"
}
```

### Null Handling

- JSON `null` deserializes to empty/nil slice
- Empty string `""` deserializes to empty slice

### String Representation

Both types implement `String()` / `Display` for easy logging:

```
StdBase64Data("hello") -> "aGVsbG8="
HexData([0xde, 0xad]) -> "dead"
```

## Use Cases

### API Payloads

Many APIs return binary data as Base64-encoded JSON strings:

```json
{
  "audio_data": "UklGRi4AAABXQVZFZm10IBAAAAABAAEA..."
}
```

### Hash Values

Cryptographic hashes are typically represented as hex:

```json
{
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}
```

### Binary Protocol Debugging

Hex encoding is useful for debugging binary protocols:

```json
{
  "raw_frame": "0102030405"
}
```

## Examples Directory

- `examples/go/encoding/` - Go usage examples (if any)
- `examples/rust/encoding/` - Rust usage examples (if any)

## Related Packages

- `minimax` - Uses Base64 for audio data in API responses
- `doubaospeech` - Uses Base64 for audio payloads
- `dashscope` - Uses Base64 for binary data
