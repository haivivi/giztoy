# JsonTime Package - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/jsontime`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/jsontime)

## Types

### Unix

```go
type Unix time.Time
```

A `time.Time` that serializes to/from Unix seconds in JSON.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NowEpoch` | `func NowEpoch() Unix` | Current time as Unix |
| `Time` | `(ep Unix) Time() time.Time` | Get underlying time.Time |
| `Before` | `(ep Unix) Before(t Unix) bool` | Is ep before t? |
| `After` | `(ep Unix) After(t Unix) bool` | Is ep after t? |
| `Equal` | `(ep Unix) Equal(t Unix) bool` | Are times equal? |
| `Sub` | `(ep Unix) Sub(t Unix) time.Duration` | Duration ep-t |
| `Add` | `(ep Unix) Add(d time.Duration) Unix` | Return ep+d |
| `IsZero` | `(ep Unix) IsZero() bool` | Is zero time? |
| `String` | `(ep Unix) String() string` | Formatted string |

### Milli

```go
type Milli time.Time
```

A `time.Time` that serializes to/from Unix milliseconds in JSON.

**Methods:** Same as Unix.

| Method | Signature | Description |
|--------|-----------|-------------|
| `NowEpochMilli` | `func NowEpochMilli() Milli` | Current time as Milli |
| `Time` | `(ep Milli) Time() time.Time` | Get underlying time.Time |
| ... | | (same operations as Unix) |

### Duration

```go
type Duration time.Duration
```

A `time.Duration` that serializes to string (e.g., "1h30m") in JSON.

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `FromDuration` | `func FromDuration(d time.Duration) *Duration` | Create Duration pointer |
| `Duration` | `(d *Duration) Duration() time.Duration` | Get underlying duration |
| `String` | `(d Duration) String() string` | Formatted string (e.g., "1h30m") |
| `Seconds` | `(d Duration) Seconds() float64` | As floating point seconds |
| `Milliseconds` | `(d Duration) Milliseconds() int64` | As integer milliseconds |

## Usage

### In Struct Fields

```go
type Event struct {
    ID        string   `json:"id"`
    CreatedAt Unix     `json:"created_at"`
    ExpiresAt Milli    `json:"expires_at"`
    Timeout   Duration `json:"timeout"`
}

event := Event{
    ID:        "evt-123",
    CreatedAt: NowEpoch(),
    ExpiresAt: NowEpochMilli(),
    Timeout:   Duration(30 * time.Second),
}

// Marshals to:
// {"id":"evt-123","created_at":1705315800,"expires_at":1705315800000,"timeout":"30s"}
```

### Duration Parsing

Duration accepts both string and integer (nanoseconds) when unmarshaling:

```go
type Config struct {
    Timeout Duration `json:"timeout"`
}

// String format
json.Unmarshal([]byte(`{"timeout":"1h30m"}`), &cfg)
fmt.Println(cfg.Timeout.Duration())  // 1h30m0s

// Integer format (nanoseconds)
json.Unmarshal([]byte(`{"timeout":5400000000000}`), &cfg)
fmt.Println(cfg.Timeout.Duration())  // 1h30m0s
```

### Time Arithmetic

```go
now := NowEpoch()
later := now.Add(24 * time.Hour)

if later.After(now) {
    diff := later.Sub(now)
    fmt.Println(diff)  // 24h0m0s
}
```

### Null Handling

```go
var d Duration
json.Unmarshal([]byte(`null`), &d)  // d remains zero value
```

## Implementation Details

### Type Aliases

All types are direct aliases, allowing easy conversion:

```go
// Unix -> time.Time
t := time.Time(myUnix)

// time.Time -> Unix
u := Unix(time.Now())

// Duration -> time.Duration
d := time.Duration(myDuration)
```

### JSON Marshal Output

| Type | Go Value | JSON Output |
|------|----------|-------------|
| Unix | `Unix(time.Now())` | `1705315800` |
| Milli | `Milli(time.Now())` | `1705315800000` |
| Duration | `Duration(90*time.Second)` | `"1m30s"` |

## Dependencies

- `time` (stdlib)
- `encoding/json` (stdlib)
