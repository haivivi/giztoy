# Audio Resampler - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/audio/resampler`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/audio/resampler)

## Types

### Resampler Interface

```go
type Resampler interface {
    io.ReadCloser
    CloseWithError(error) error
}
```

### Format

```go
type Format struct {
    SampleRate int
    Stereo     bool
}
```

**Helper Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `sampleBytes` | `() int` | Bytes per sample (2 or 4) |
| `channels` | `() int` | Channel count (1 or 2) |

### Soxr

```go
type Soxr struct {
    srcFmt Format
    dstFmt Format
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `New` | `func New(src io.Reader, srcFmt, dstFmt Format) (Resampler, error)` | Create resampler |
| `Read` | `(r *Soxr) Read(p []byte) (int, error)` | Read resampled data |
| `Close` | `(r *Soxr) Close() error` | Release resources |
| `CloseWithError` | `(r *Soxr) CloseWithError(err error) error` | Close with custom error |

### SampleReader

Utility for reading complete samples from a reader that may return partial data.

```go
// Internal helper - ensures reads are sample-aligned
sr := newSampleReader(reader, sampleBytes)
```

## Usage

### Basic Resampling

```go
import "github.com/haivivi/giztoy/pkg/audio/resampler"

// 24kHz to 16kHz
srcFmt := resampler.Format{SampleRate: 24000, Stereo: false}
dstFmt := resampler.Format{SampleRate: 16000, Stereo: false}

rs, err := resampler.New(audioSource, srcFmt, dstFmt)
if err != nil {
    return err
}
defer rs.Close()

// Read resampled data
buf := make([]byte, 3200) // 100ms at 16kHz mono
for {
    n, err := rs.Read(buf)
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    // Process buf[:n]
}
```

### Stereo to Mono

```go
srcFmt := resampler.Format{SampleRate: 48000, Stereo: true}
dstFmt := resampler.Format{SampleRate: 16000, Stereo: false}

rs, _ := resampler.New(stereoSource, srcFmt, dstFmt)
// Output is downmixed mono at 16kHz
```

### Mono to Stereo

```go
srcFmt := resampler.Format{SampleRate: 16000, Stereo: false}
dstFmt := resampler.Format{SampleRate: 48000, Stereo: true}

rs, _ := resampler.New(monoSource, srcFmt, dstFmt)
// Output is duplicated stereo at 48kHz
```

### Pipeline with Decoder

```go
// MP3 (24kHz) → Decode → Resample → Mixer (16kHz)
mp3Data := bytes.NewReader(encodedData)
decoder, _ := mp3.NewDecoder(mp3Data)

srcFmt := resampler.Format{SampleRate: 24000, Stereo: false}
dstFmt := resampler.Format{SampleRate: 16000, Stereo: false}

rs, _ := resampler.New(decoder, srcFmt, dstFmt)

// Feed to mixer
track.WriteFrom(rs)
```

## CGO Details

```go
/*
#cgo pkg-config: soxr
#include <soxr.h>
*/
import "C"

// Quality setting (hardcoded to HQ)
qSpec := C.soxr_quality_spec(C.SOXR_HQ, 0)

// I/O format (int16 for both input and output)
ioSpec := C.soxr_io_spec(C.SOXR_INT16_I, C.SOXR_INT16_I)
```

## Error Handling

- `io.EOF`: Source exhausted, resampler flushed
- `io.ErrShortBuffer`: Output buffer too small for one sample
- `io.ErrClosedPipe`: Resampler was closed

## Thread Safety

- `Read` is NOT safe for concurrent calls
- `Close`/`CloseWithError` can be called from any goroutine
