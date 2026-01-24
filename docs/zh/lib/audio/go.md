# Audio Package - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/audio`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/audio)

The main `audio` package is an umbrella for sub-packages. Import specific packages directly.

## Sub-packages

### pcm (PCM Audio)

```go
import "github.com/haivivi/giztoy/pkg/audio/pcm"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Format` | Audio format (sample rate, channels, depth) |
| `Chunk` | Interface for audio data chunks |
| `DataChunk` | Raw audio data chunk |
| `SilenceChunk` | Silence generator |
| `Mixer` | Multi-track audio mixer |
| `Track` | Single audio track in mixer |
| `TrackCtrl` | Track control (gain, play/stop) |

### codec/opus (Opus Codec)

```go
import "github.com/haivivi/giztoy/pkg/audio/codec/opus"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Encoder` | Opus encoder (wraps libopus) |
| `Decoder` | Opus decoder (wraps libopus) |
| `Frame` | Raw Opus frame data (`[]byte`) |
| `TOC` | Table of Contents byte parser |
| `FrameDuration` | Frame duration enum |

### codec/mp3 (MP3 Codec)

```go
import "github.com/haivivi/giztoy/pkg/audio/codec/mp3"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Encoder` | MP3 encoder (wraps LAME) |
| `Decoder` | MP3 decoder (wraps minimp3) |

### codec/ogg (OGG Container)

```go
import "github.com/haivivi/giztoy/pkg/audio/codec/ogg"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Encoder` | OGG page encoder |
| `Stream` | OGG logical bitstream |
| `Sync` | OGG page synchronizer |

### resampler (Sample Rate Conversion)

```go
import "github.com/haivivi/giztoy/pkg/audio/resampler"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Resampler` | Interface for sample rate conversion |
| `Soxr` | libsoxr-based resampler |
| `Format` | Source/destination format |

### opusrt (Realtime Opus)

```go
import "github.com/haivivi/giztoy/pkg/audio/opusrt"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Buffer` | Jitter buffer for out-of-order frames |
| `RealtimeBuffer` | Real-time playback simulation |
| `StampedFrame` | Opus frame with timestamp |
| `OggReader` | Read Opus from OGG container |
| `OggWriter` | Write Opus to OGG container |

### portaudio (Audio I/O)

```go
import "github.com/haivivi/giztoy/pkg/audio/portaudio"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Stream` | Audio input/output stream |

### songs (Built-in Melodies)

```go
import "github.com/haivivi/giztoy/pkg/audio/songs"
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Song` | Melody definition |
| `Note` | Musical note |

## Usage Examples

### PCM Mixer

```go
import "github.com/haivivi/giztoy/pkg/audio/pcm"

// Create mixer
mixer := pcm.NewMixer(pcm.L16Mono16K, pcm.WithAutoClose())

// Create track
track, ctrl, _ := mixer.CreateTrack(pcm.WithTrackLabel("voice"))

// Write audio to track
track.Write(audioData)

// Adjust gain
ctrl.SetGain(0.8)

// Read mixed output
buf := make([]byte, 1600) // 50ms at 16kHz
mixer.Read(buf)
```

### Opus Encoding

```go
import "github.com/haivivi/giztoy/pkg/audio/codec/opus"

// Create encoder
enc, _ := opus.NewVoIPEncoder(16000, 1)
defer enc.Close()

enc.SetBitrate(24000)

// Encode PCM to Opus
pcmData := make([]int16, 320) // 20ms at 16kHz
frame, _ := enc.Encode(pcmData, 320)
```

### Sample Rate Conversion

```go
import "github.com/haivivi/giztoy/pkg/audio/resampler"

srcFmt := resampler.Format{SampleRate: 24000, Stereo: false}
dstFmt := resampler.Format{SampleRate: 16000, Stereo: false}

rs, _ := resampler.New(audioReader, srcFmt, dstFmt)
defer rs.Close()

// Read resampled data
io.Copy(output, rs)
```

### Realtime Opus Buffer

```go
import "github.com/haivivi/giztoy/pkg/audio/opusrt"

// Create jitter buffer (2 minute capacity)
buf := opusrt.NewBuffer(2 * time.Minute)

// Write stamped frames (can arrive out of order)
buf.Write(stampedFrameData)

// Read in order
frame, loss, _ := buf.Frame()
if loss > 0 {
    // Use decoder PLC for lost frames
}
```

## CGO Dependencies

All codec packages use CGO to bind native libraries:

```go
// Example: opus encoder
/*
#cgo pkg-config: opus
#include <opus.h>
*/
import "C"
```

Build requirements:
- `pkg-config` for native builds
- Bazel `cdeps` for Bazel builds
