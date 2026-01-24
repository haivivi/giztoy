# Audio Package - Rust Implementation

Crate: `giztoy-audio`

📚 [Rust Documentation](https://docs.rs/giztoy-audio)

## Modules

### pcm (PCM Audio)

```rust
use giztoy_audio::pcm::{Format, FormatExt, Chunk, DataChunk, SilenceChunk, Mixer};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Format` | Audio format enum (re-exported from resampler) |
| `FormatExt` | Extension trait for chunk creation |
| `Chunk` | Trait for audio data chunks |
| `DataChunk` | Raw audio data chunk |
| `SilenceChunk` | Silence generator |
| `Mixer` | Multi-track audio mixer |
| `Track` | Audio track writer |
| `TrackCtrl` | Track control |
| `AtomicF32` | Atomic float for gain control |

### codec::opus (Opus Codec)

```rust
use giztoy_audio::codec::opus::{Encoder, Decoder, Application, Frame, TOC};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Encoder` | Opus encoder (wraps libopus) |
| `Decoder` | Opus decoder |
| `Application` | Encoder application type enum |
| `Frame` | Raw Opus frame data |
| `TOC` | Table of Contents parser |
| `FrameDuration` | Frame duration enum |

### codec::mp3 (MP3 Codec)

```rust
use giztoy_audio::codec::mp3::{Encoder, Decoder};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Encoder` | MP3 encoder (wraps LAME) |
| `Decoder` | MP3 decoder (wraps minimp3) |

### codec::ogg (OGG Container)

```rust
use giztoy_audio::codec::ogg::{Encoder, Stream, Sync, Page};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Encoder` | OGG page encoder |
| `Stream` | OGG logical bitstream |
| `Sync` | OGG page synchronizer |
| `Page` | OGG page data |

### resampler (Sample Rate Conversion)

```rust
use giztoy_audio::resampler::{Soxr, Format};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Soxr` | libsoxr-based resampler |
| `Format` | Audio format (sample rate, stereo flag) |

### opusrt (Realtime Opus)

```rust
use giztoy_audio::opusrt::{Buffer, StampedFrame, EpochMillis};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Buffer` | Jitter buffer for frame reordering |
| `StampedFrame` | Opus frame with timestamp |
| `EpochMillis` | Millisecond timestamp |

⚠️ **Note:** Rust opusrt is missing OGG Reader/Writer compared to Go.

### songs (Built-in Melodies)

```rust
use giztoy_audio::songs::{Song, Note, Catalog};
```

**Key Types:**

| Type | Description |
|------|-------------|
| `Song` | Melody definition |
| `Note` | Musical note |
| `Catalog` | Built-in song collection |

## Usage Examples

### PCM Format

```rust
use giztoy_audio::pcm::{Format, FormatExt};
use std::time::Duration;

let format = Format::L16Mono16K;

// Calculate bytes for duration
let bytes = format.bytes_in_duration(Duration::from_millis(100));
assert_eq!(bytes, 3200); // 1600 samples * 2 bytes

// Create chunks
let silence = format.silence_chunk(Duration::from_millis(100));
let data = format.data_chunk(vec![0u8; 3200]);
```

### Opus Encoding

```rust
use giztoy_audio::codec::opus::{Encoder, Application};

let mut encoder = Encoder::new(16000, 1, Application::VoIP)?;
encoder.set_bitrate(24000)?;

// Encode PCM to Opus
let pcm: Vec<i16> = vec![0; 320]; // 20ms at 16kHz
let frame = encoder.encode(&pcm, 320)?;
```

### Sample Rate Conversion

```rust
use giztoy_audio::resampler::{Soxr, Format};

let src_fmt = Format { sample_rate: 24000, stereo: false };
let dst_fmt = Format { sample_rate: 16000, stereo: false };

let mut resampler = Soxr::new(src_fmt, dst_fmt)?;

// Process audio data
let output = resampler.process(&input_pcm)?;
```

### Mixer

```rust
use giztoy_audio::pcm::{Format, Mixer, MixerOptions};

let mut mixer = Mixer::new(Format::L16Mono16K, MixerOptions::default());

// Create track
let (track, ctrl) = mixer.create_track(None)?;

// Write audio
track.write(&audio_data)?;

// Adjust gain
ctrl.set_gain(0.8);

// Read mixed output
let mut buf = vec![0u8; 3200];
mixer.read(&mut buf)?;
```

## FFI Bindings

Rust uses custom FFI modules for native library bindings:

```rust
// Example: codec/opus/ffi.rs
extern "C" {
    fn opus_encoder_create(
        fs: i32,
        channels: i32,
        application: i32,
        error: *mut i32,
    ) -> *mut OpusEncoder;
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| Format definition | In `pcm/` | In `resampler/`, re-exported by `pcm/` |
| opusrt OGG R/W | ✅ | ❌ Missing |
| portaudio | ✅ | ❌ Not implemented |
| Mixer thread-safety | `sync.Mutex` | `std::sync::Mutex` |
| FFI error handling | CGO strerror | Custom error types |
