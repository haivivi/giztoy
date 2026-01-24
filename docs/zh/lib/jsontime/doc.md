# JsonTime Package

JSON-serializable time types for API integrations.

## Design Goals

1. **API Compatibility**: Many APIs use Unix timestamps instead of ISO 8601 strings
2. **Type Safety**: Distinct types prevent mixing seconds/milliseconds
3. **Bidirectional**: Both serialization and deserialization supported

## Types

| Type | JSON Format | Example | Use Case |
|------|-------------|---------|----------|
| `Unix` | Integer (seconds) | `1705315800` | General timestamps |
| `Milli` | Integer (milliseconds) | `1705315800000` | High-precision timestamps |
| `Duration` | String or Integer | `"1h30m"` or `5400000000000` | Time intervals |

## Features

### Unix Timestamps

Many APIs use Unix epoch timestamps rather than ISO 8601:

```json
{
  "created_at": 1705315800,
  "updated_at": 1705316000
}
```

### Millisecond Precision

JavaScript/browser APIs often use milliseconds:

```json
{
  "timestamp": 1705315800000,
  "expires_at": 1705316000000
}
```

### Flexible Duration Parsing

Duration supports both human-readable strings and raw nanoseconds:

```json
{
  "timeout": "30s",
  "interval": "1h30m",
  "precise_delay": 5000000000
}
```

## Time Operations

Both `Unix` and `Milli` support common time operations:

| Operation | Description |
|-----------|-------------|
| `Before(t)` | Is this time before t? |
| `After(t)` | Is this time after t? |
| `Equal(t)` | Are these times equal? |
| `Sub(t)` | Duration between times |
| `Add(d)` | Add duration to time |
| `IsZero()` | Is this the zero time? |

## Duration String Format

The Duration type uses Go-style duration strings:

| Unit | Symbol | Example |
|------|--------|---------|
| Hours | `h` | `2h` |
| Minutes | `m` | `30m` |
| Seconds | `s` | `45s` |
| Combined | | `1h30m45s` |

## Examples Directory

- `examples/go/jsontime/` - Go usage examples (if any)
- `examples/rust/jsontime/` - Rust usage examples (if any)

## Related Packages

- `minimax` - Uses Milli for timestamps in API responses
- `doubaospeech` - Uses Unix/Milli for audio timestamps
- `dashscope` - Uses Duration for timeout configuration
