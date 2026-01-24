# Audio PCM - Rust Implementation

Crate: `giztoy-audio`

📚 [Rust Documentation](https://docs.rs/giztoy-audio) (module `pcm`)

## Types

### Format

```rust
// Re-exported from resampler module
pub use crate::resampler::format::Format;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Format {
    L16Mono16K,  // 16000 Hz, mono, 16-bit
    L16Mono24K,  // 24000 Hz, mono, 16-bit
    L16Mono48K,  // 48000 Hz, mono, 16-bit
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `sample_rate` | `fn sample_rate(&self) -> u32` | Get sample rate in Hz |
| `channels` | `fn channels(&self) -> u32` | Get channel count |
| `depth` | `fn depth(&self) -> u32` | Get bit depth |
| `samples` | `fn samples(&self, bytes: u64) -> u64` | Bytes to samples |
| `samples_in_duration` | `fn samples_in_duration(&self, d: Duration) -> u64` | Duration to samples |
| `bytes_in_duration` | `fn bytes_in_duration(&self, d: Duration) -> u64` | Duration to bytes |
| `duration` | `fn duration(&self, bytes: u64) -> Duration` | Bytes to duration |
| `bits_rate` | `fn bits_rate(&self) -> u32` | Get bits per second |
| `bytes_rate` | `fn bytes_rate(&self) -> u32` | Get bytes per second |

### FormatExt Trait

```rust
pub trait FormatExt {
    fn silence_chunk(&self, duration: Duration) -> SilenceChunk;
    fn data_chunk(&self, data: Vec<u8>) -> DataChunk;
    fn data_chunk_from_samples(&self, samples: &[i16]) -> DataChunk;
}

impl FormatExt for Format { /* ... */ }
```

### Chunk Trait

```rust
pub trait Chunk {
    fn len(&self) -> u64;
    fn format(&self) -> Format;
    fn write_to<W: Write>(&self, writer: &mut W) -> io::Result<u64>;
}
```

### DataChunk

```rust
pub struct DataChunk {
    data: Vec<u8>,
    format: Format,
}
```

### SilenceChunk

```rust
pub struct SilenceChunk {
    duration: Duration,
    len: u64,
    format: Format,
}
```

### Mixer

```rust
pub struct Mixer {
    output: Format,
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(output: Format, opts: MixerOptions) -> Self` | Create mixer |
| `output` | `fn output(&self) -> Format` | Get output format |
| `create_track` | `fn create_track(&mut self, opts: Option<TrackOptions>) -> Result<(Track, TrackCtrl)>` | Create track |
| `read` | `fn read(&mut self, buf: &mut [u8]) -> Result<usize>` | Read mixed audio |
| `close_write` | `fn close_write(&mut self) -> Result<()>` | Stop accepting tracks |
| `close` | `fn close(&mut self) -> Result<()>` | Close mixer |

### MixerOptions

```rust
#[derive(Default)]
pub struct MixerOptions {
    pub auto_close: bool,
    pub silence_gap: Option<Duration>,
}
```

### Track & TrackCtrl

```rust
pub struct Track {
    // ...
}

impl Track {
    pub fn write(&mut self, data: &[u8]) -> io::Result<usize>;
    pub fn close(&mut self) -> Result<()>;
}

pub struct TrackCtrl {
    // ...
}

impl TrackCtrl {
    pub fn gain(&self) -> f32;
    pub fn set_gain(&self, gain: f32);
    pub fn close_write(&mut self) -> Result<()>;
}
```

### AtomicF32

```rust
pub struct AtomicF32 {
    bits: AtomicU32,
}

impl AtomicF32 {
    pub fn new(value: f32) -> Self;
    pub fn load(&self) -> f32;
    pub fn store(&self, value: f32);
}
```

## Usage

### Format Calculations

```rust
use giztoy_audio::pcm::Format;
use std::time::Duration;

let format = Format::L16Mono16K;

// Calculate for 100ms
let bytes = format.bytes_in_duration(Duration::from_millis(100));
// bytes = 3200

let samples = format.samples_in_duration(Duration::from_millis(100));
// samples = 1600

// Reverse calculation
let duration = format.duration(3200);
// duration = 100ms
```

### Creating Chunks

```rust
use giztoy_audio::pcm::{Format, FormatExt};
use std::time::Duration;

let format = Format::L16Mono16K;

// Data chunk from bytes
let data = vec![0u8; 3200]; // 100ms
let chunk = format.data_chunk(data);

// Data chunk from samples
let samples: Vec<i16> = vec![0; 1600];
let chunk = format.data_chunk_from_samples(&samples);

// Silence chunk
let silence = format.silence_chunk(Duration::from_millis(100));
```

### Mixer Example

```rust
use giztoy_audio::pcm::{Format, Mixer, MixerOptions};

let opts = MixerOptions {
    auto_close: true,
    ..Default::default()
};
let mut mixer = Mixer::new(Format::L16Mono16K, opts);

// Create voice track
let (mut voice, voice_ctrl) = mixer.create_track(None)?;
voice_ctrl.set_gain(1.0);

// Create music track
let (mut music, music_ctrl) = mixer.create_track(None)?;
music_ctrl.set_gain(0.3);

// Write audio (in separate threads)
std::thread::spawn(move || {
    voice.write(&voice_data).unwrap();
    voice.close().unwrap();
});

// Read mixed output
let mut buf = vec![0u8; 3200];
loop {
    match mixer.read(&mut buf) {
        Ok(0) => break,
        Ok(n) => { /* process buf[..n] */ }
        Err(e) => break,
    }
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| Format location | In `pcm/` | In `resampler/`, re-exported |
| Chunk creation | `format.DataChunk()` | `format.data_chunk()` via trait |
| Mixer options | Variadic options | Options struct |
| Track creation | Returns 3 values | Returns tuple |
| Atomic float | Custom `AtomicFloat32` | Custom `AtomicF32` |
