# Audio PCM - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/audio/pcm`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/audio/pcm)

## Types

### Format

```go
type Format int

const (
    L16Mono16K Format = iota  // audio/L16; rate=16000; channels=1
    L16Mono24K                 // audio/L16; rate=24000; channels=1
    L16Mono48K                 // audio/L16; rate=48000; channels=1
)
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `SampleRate` | `() int` | Get sample rate in Hz |
| `Channels` | `() int` | Get channel count |
| `Depth` | `() int` | Get bit depth |
| `Samples` | `(bytes int64) int64` | Bytes to samples |
| `SamplesInDuration` | `(d time.Duration) int64` | Duration to samples |
| `BytesInDuration` | `(d time.Duration) int64` | Duration to bytes |
| `Duration` | `(bytes int64) time.Duration` | Bytes to duration |
| `BitsRate` | `() int` | Get bits per second |
| `BytesRate` | `() int` | Get bytes per second |
| `SilenceChunk` | `(d time.Duration) Chunk` | Create silence chunk |
| `DataChunk` | `(data []byte) Chunk` | Create data chunk |
| `ReadChunk` | `(r io.Reader, d time.Duration) (Chunk, error)` | Read chunk from reader |
| `String` | `() string` | Human-readable format |

### Chunk Interface

```go
type Chunk interface {
    Len() int64
    Format() Format
    WriteTo(w io.Writer) (int64, error)
}
```

### DataChunk

```go
type DataChunk struct {
    Data []byte
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Len` | `() int64` | Data length |
| `Format` | `() Format` | Audio format |
| `ReadFrom` | `(r io.Reader) (int64, error)` | Read into chunk |
| `WriteTo` | `(w io.Writer) (int64, error)` | Write chunk data |

### SilenceChunk

```go
type SilenceChunk struct {
    Duration time.Duration
}
```

Writes zeros without allocating a full buffer.

### Mixer

```go
type Mixer struct {
    output    Format
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewMixer` | `func NewMixer(output Format, opts ...MixerOption) *Mixer` | Create mixer |
| `Output` | `() Format` | Get output format |
| `CreateTrack` | `(opts ...TrackOption) (Track, *TrackCtrl, error)` | Create track |
| `Read` | `(p []byte) (int, error)` | Read mixed audio |
| `CloseWrite` | `() error` | Stop accepting new tracks |
| `Close` | `() error` | Close mixer |
| `CloseWithError` | `(err error) error` | Close with error |

**Options:**

```go
// Auto-close when all tracks are done
mixer := NewMixer(L16Mono16K, WithAutoClose())

// Close after silence duration
mixer := NewMixer(L16Mono16K, WithSilenceGap(5*time.Second))
```

### Track & TrackCtrl

```go
type Track interface {
    Write(p []byte) (int, error)
    WriteChunk(c Chunk) (int64, error)
    Close() error
    CloseWithError(err error) error
}

type TrackCtrl struct {
    // ...
}
```

**TrackCtrl Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `Gain` | `() float32` | Get current gain |
| `SetGain` | `(g float32)` | Set gain (0.0 - 1.0+) |
| `CloseWrite` | `() error` | Stop writing |
| `CloseWithError` | `(err error) error` | Close with error |

## Usage

### Format Calculations

```go
format := pcm.L16Mono16K

// Calculate for 100ms
bytes := format.BytesInDuration(100 * time.Millisecond)
// bytes = 3200

samples := format.SamplesInDuration(100 * time.Millisecond)
// samples = 1600

// Reverse calculation
duration := format.Duration(3200)
// duration = 100ms
```

### Creating Chunks

```go
format := pcm.L16Mono16K

// Data chunk
data := make([]byte, 3200) // 100ms
chunk := format.DataChunk(data)

// Silence chunk (no allocation)
silence := format.SilenceChunk(100 * time.Millisecond)

// Read chunk from io.Reader
chunk, err := format.ReadChunk(reader, 100*time.Millisecond)
```

### Mixer Example

```go
mixer := pcm.NewMixer(pcm.L16Mono16K, pcm.WithAutoClose())

// Create voice track
voice, voiceCtrl, _ := mixer.CreateTrack(pcm.WithTrackLabel("voice"))
voiceCtrl.SetGain(1.0)

// Create music track at lower volume
music, musicCtrl, _ := mixer.CreateTrack(pcm.WithTrackLabel("music"))
musicCtrl.SetGain(0.3)

// Write audio to tracks (in separate goroutines)
go func() {
    voice.Write(voiceData)
    voice.Close()
}()
go func() {
    music.Write(musicData)
    music.Close()
}()

// Read mixed output
buf := make([]byte, 3200)
for {
    n, err := mixer.Read(buf)
    if err == io.EOF {
        break
    }
    // Process mixed audio in buf[:n]
}
```
