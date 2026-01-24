# Audio Codec - Go Implementation

## opus/ Package

Import: `github.com/haivivi/giztoy/pkg/audio/codec/opus`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/audio/codec/opus)

### Types

#### Encoder

```go
type Encoder struct {
    sampleRate int
    channels   int
    cEnc       *C.OpusEncoder
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewEncoder` | `func NewEncoder(sampleRate, channels, application int) (*Encoder, error)` | Create encoder |
| `NewVoIPEncoder` | `func NewVoIPEncoder(sampleRate, channels int) (*Encoder, error)` | Voice-optimized encoder |
| `NewAudioEncoder` | `func NewAudioEncoder(sampleRate, channels int) (*Encoder, error)` | Music-optimized encoder |
| `Close` | `(e *Encoder) Close()` | Release resources |
| `Encode` | `(e *Encoder) Encode(pcm []int16, frameSize int) (Frame, error)` | Encode PCM to Opus |
| `EncodeBytes` | `(e *Encoder) EncodeBytes(pcm []byte, frameSize int) (Frame, error)` | Encode from bytes |
| `EncodeTo` | `(e *Encoder) EncodeTo(pcm []int16, frameSize int, buf []byte) (int, error)` | Encode to buffer |
| `SetBitrate` | `(e *Encoder) SetBitrate(bitrate int) error` | Set target bitrate |
| `SetComplexity` | `(e *Encoder) SetComplexity(complexity int) error` | Set CPU complexity |
| `FrameSize20ms` | `(e *Encoder) FrameSize20ms() int` | Get 20ms frame size |

**Application Constants:**

```go
const (
    ApplicationVoIP              = int(C.OPUS_APPLICATION_VOIP)
    ApplicationAudio             = int(C.OPUS_APPLICATION_AUDIO)
    ApplicationRestrictedLowdelay = int(C.OPUS_APPLICATION_RESTRICTED_LOWDELAY)
)
```

#### Decoder

```go
type Decoder struct {
    sampleRate int
    channels   int
    cDec       *C.OpusDecoder
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewDecoder` | `func NewDecoder(sampleRate, channels int) (*Decoder, error)` | Create decoder |
| `Close` | `(d *Decoder) Close()` | Release resources |
| `Decode` | `(d *Decoder) Decode(f Frame) ([]byte, error)` | Decode Opus to PCM |
| `DecodeTo` | `(d *Decoder) DecodeTo(f Frame, buf []int16) (int, error)` | Decode to buffer |
| `DecodePLC` | `(d *Decoder) DecodePLC(samples int) ([]byte, error)` | Packet loss concealment |

#### Frame & TOC

```go
type Frame []byte

type TOC struct {
    Config      int           // 0-31 configuration
    StereoFlag  bool          // True for stereo
    FrameCount  int           // 1-48 frames
    Duration    FrameDuration // Per-frame duration
}
```

### Usage

```go
// Create encoder
enc, _ := opus.NewVoIPEncoder(16000, 1)
defer enc.Close()

enc.SetBitrate(24000)
enc.SetComplexity(5)

// Encode 20ms of audio
pcm := make([]int16, enc.FrameSize20ms())
// ... fill pcm with audio data ...
frame, _ := enc.Encode(pcm, enc.FrameSize20ms())

// Create decoder
dec, _ := opus.NewDecoder(16000, 1)
defer dec.Close()

// Decode
pcmOut, _ := dec.Decode(frame)

// Handle packet loss
plcOut, _ := dec.DecodePLC(320) // Generate 20ms of PLC audio
```

---

## mp3/ Package

Import: `github.com/haivivi/giztoy/pkg/audio/codec/mp3`

### Encoder (LAME)

```go
enc, _ := mp3.NewEncoder(sampleRate, channels, bitrate)
defer enc.Close()

mp3Data, _ := enc.Encode(pcmData)
```

### Decoder (minimp3)

```go
dec, _ := mp3.NewDecoder()
defer dec.Close()

pcmData, _ := dec.Decode(mp3Data)
```

---

## ogg/ Package

Import: `github.com/haivivi/giztoy/pkg/audio/codec/ogg`

### Stream

```go
// Create bitstream
stream := ogg.NewStream(serialNo)

// Add Opus packets
stream.PacketIn(packet)

// Get pages
for {
    page, ok := stream.PageOut()
    if !ok {
        break
    }
    // Write page to output
}
```

### Sync (for reading)

```go
sync := ogg.NewSync()

// Feed data
sync.Buffer(data)

// Read pages
for {
    page, ok := sync.PageOut()
    if !ok {
        break
    }
    // Process page
}
```
