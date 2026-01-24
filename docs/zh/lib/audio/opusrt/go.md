# Audio OpusRT - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/audio/opusrt`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/audio/opusrt)

## Types

### EpochMillis

```go
type EpochMillis int64
```

**Functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `Now` | `func Now() EpochMillis` | Current time as epoch millis |
| `FromDuration` | `func FromDuration(d time.Duration) EpochMillis` | Duration to millis |

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Duration` | `(e EpochMillis) Duration() time.Duration` | To time.Duration |
| `Time` | `(e EpochMillis) Time() time.Time` | To time.Time |

### Frame

```go
type Frame []byte
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Duration` | `(f Frame) Duration() time.Duration` | Frame duration from TOC |
| `Clone` | `(f Frame) Clone() Frame` | Deep copy |

### StampedFrame

```go
// Binary format: 8-byte timestamp (big-endian) + Opus frame
func FromStamped(data []byte) (Frame, EpochMillis, bool)
func ToStamped(frame Frame, stamp EpochMillis) []byte
```

### Buffer

```go
type Buffer struct {
    Duration EpochMillis  // Max buffer duration
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewBuffer` | `func NewBuffer(d time.Duration) *Buffer` | Create buffer |
| `Append` | `(buf *Buffer) Append(frame Frame, stamp EpochMillis) error` | Add frame |
| `Write` | `(buf *Buffer) Write(stamped []byte) (int, error)` | io.Writer for stamped frames |
| `Frame` | `(buf *Buffer) Frame() (Frame, time.Duration, error)` | Get next frame |
| `Len` | `(buf *Buffer) Len() int` | Frame count |
| `Buffered` | `(buf *Buffer) Buffered() time.Duration` | Buffered duration |
| `Reset` | `(buf *Buffer) Reset()` | Clear buffer |

### RealtimeBuffer

```go
type RealtimeBuffer struct {
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `RealtimeFrom` | `func RealtimeFrom(buf *Buffer) *RealtimeBuffer` | Create from Buffer |
| `Write` | `(rtb *RealtimeBuffer) Write(stamped []byte) (int, error)` | io.Writer |
| `Frame` | `(rtb *RealtimeBuffer) Frame() (Frame, time.Duration, error)` | Get next frame/loss |
| `CloseWrite` | `(rtb *RealtimeBuffer) CloseWrite() error` | Signal end of input |
| `Close` | `(rtb *RealtimeBuffer) Close() error` | Close buffer |
| `Reset` | `(rtb *RealtimeBuffer) Reset()` | Clear buffer |

### OggWriter

```go
type OggWriter struct {
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewOggWriter` | `func NewOggWriter(w io.Writer, sampleRate int) (*OggWriter, error)` | Create writer |
| `Write` | `(ow *OggWriter) Write(frame Frame) error` | Write frame |
| `Close` | `(ow *OggWriter) Close() error` | Finalize and close |

### OggReader

```go
type OggReader struct {
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|----------|-----------|-------------|
| `NewOggReader` | `func NewOggReader(r io.Reader) (*OggReader, error)` | Create reader |
| `Frame` | `(or *OggReader) Frame() (Frame, time.Duration, error)` | Read next frame |

## Usage

### Basic Jitter Buffer

```go
import "github.com/haivivi/giztoy/pkg/audio/opusrt"

// Create buffer (2 minute capacity)
buf := opusrt.NewBuffer(2 * time.Minute)

// Write stamped frames (from network)
buf.Write(stampedData)

// Read in order
for {
    frame, loss, err := buf.Frame()
    if err == io.EOF {
        break
    }
    if loss > 0 {
        // Handle packet loss with PLC
        pcm := decoder.DecodePLC(int(loss / 20 * time.Millisecond * 320))
        play(pcm)
    } else {
        pcm := decoder.Decode(frame)
        play(pcm)
    }
}
```

### Realtime Playback

```go
// Create realtime buffer
buf := opusrt.NewBuffer(2 * time.Minute)
rtb := opusrt.RealtimeFrom(buf)

// Write frames (from another goroutine)
go func() {
    for data := range networkData {
        rtb.Write(data)
    }
    rtb.CloseWrite()
}()

// Read at real-time pace
for {
    frame, loss, err := rtb.Frame()
    if err == io.EOF {
        break
    }
    // Frames arrive at correct timing
}
```

### OGG File Writing

```go
file, _ := os.Create("output.ogg")
writer, _ := opusrt.NewOggWriter(file, 16000)
defer writer.Close()

for _, frame := range opusFrames {
    writer.Write(frame)
}
```

### OGG File Reading

```go
file, _ := os.Open("input.ogg")
reader, _ := opusrt.NewOggReader(file)

for {
    frame, _, err := reader.Frame()
    if err == io.EOF {
        break
    }
    // Process frame
}
```

## Error Types

```go
var ErrDisorderedPacket = errors.New("opusrt: disordered packet")
var ErrInvalidFrame = errors.New("opusrt: invalid frame")
var ErrDone = errors.New("opusrt: done")
```

## Thread Safety

- `Buffer`: Safe for concurrent Append/Frame calls
- `RealtimeBuffer`: Safe for concurrent Write/Frame calls
- `OggWriter/OggReader`: NOT thread-safe
