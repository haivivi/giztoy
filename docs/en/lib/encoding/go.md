# Encoding Package - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/encoding`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/encoding)

## Types

### StdBase64Data

```go
type StdBase64Data []byte
```

A byte slice that serializes to/from standard Base64 in JSON.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `MarshalJSON` | `() ([]byte, error)` | Encode to JSON Base64 string |
| `UnmarshalJSON` | `(data []byte) error` | Decode from JSON Base64 string |
| `String` | `() string` | Return Base64-encoded string |

### HexData

```go
type HexData []byte
```

A byte slice that serializes to/from hexadecimal in JSON.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `MarshalJSON` | `() ([]byte, error)` | Encode to JSON hex string |
| `UnmarshalJSON` | `(data []byte) error` | Decode from JSON hex string |
| `String` | `() string` | Return hex-encoded string |

## Usage

### In Struct Fields

```go
type Message struct {
    ID      string        `json:"id"`
    Payload StdBase64Data `json:"payload"`
    Hash    HexData       `json:"hash"`
}

msg := Message{
    ID:      "msg-123",
    Payload: StdBase64Data([]byte("hello world")),
    Hash:    HexData([]byte{0xab, 0xcd, 0xef}),
}

// Marshals to:
// {"id":"msg-123","payload":"aGVsbG8gd29ybGQ=","hash":"abcdef"}
data, _ := json.Marshal(msg)
```

### Standalone Encoding

```go
// Base64
data := StdBase64Data([]byte("hello"))
fmt.Println(data.String())  // "aGVsbG8="

// Hex
hash := HexData([]byte{0xde, 0xad})
fmt.Println(hash.String())  // "dead"
```

### Null Handling

```go
var data StdBase64Data
json.Unmarshal([]byte(`null`), &data)  // data is nil

json.Unmarshal([]byte(`""`), &data)    // data is []byte{}
```

## Implementation Details

### UnmarshalJSON Logic

Both types handle multiple JSON input types:

```go
func (b *StdBase64Data) UnmarshalJSON(data []byte) error {
    switch data[0] {
    case 'n':  // null
        return nil
    case '"':  // string
        // decode Base64
    default:
        return error
    }
}
```

### Direct Slice Alias

Go implementation uses direct type alias `type StdBase64Data []byte`, which means:
- No wrapper overhead
- Can be cast directly to/from `[]byte`
- Shares underlying array with original slice

## Dependencies

- `encoding/base64` (stdlib)
- `encoding/hex` (stdlib)
