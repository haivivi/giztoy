# Audio Resampler - Rust Implementation

Crate: `giztoy-audio`

📚 [Rust Documentation](https://docs.rs/giztoy-audio) (module `resampler`)

## Types

### Format

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Format {
    pub sample_rate: u32,
    pub stereo: bool,
}

impl Format {
    pub const L16Mono16K: Self = Self { sample_rate: 16000, stereo: false };
    pub const L16Mono24K: Self = Self { sample_rate: 24000, stereo: false };
    pub const L16Mono48K: Self = Self { sample_rate: 48000, stereo: false };
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `sample_rate` | `fn sample_rate(&self) -> u32` | Get sample rate |
| `channels` | `fn channels(&self) -> u32` | Get channel count (1 or 2) |
| `depth` | `fn depth(&self) -> u32` | Always 16 |
| `samples` | `fn samples(&self, bytes: u64) -> u64` | Bytes to samples |
| `samples_in_duration` | `fn samples_in_duration(&self, d: Duration) -> u64` | Duration to samples |
| `bytes_in_duration` | `fn bytes_in_duration(&self, d: Duration) -> u64` | Duration to bytes |
| `duration` | `fn duration(&self, bytes: u64) -> Duration` | Bytes to duration |
| `bits_rate` | `fn bits_rate(&self) -> u32` | Bits per second |
| `bytes_rate` | `fn bytes_rate(&self) -> u32` | Bytes per second |

### Soxr

```rust
pub struct Soxr {
    src_fmt: Format,
    dst_fmt: Format,
    soxr: *mut ffi::soxr,
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(src_fmt: Format, dst_fmt: Format) -> Result<Self>` | Create resampler |
| `process` | `fn process(&mut self, input: &[u8]) -> Result<Vec<u8>>` | Resample data |
| `flush` | `fn flush(&mut self) -> Result<Vec<u8>>` | Flush remaining data |

### SampleReader

```rust
pub struct SampleReader<R> {
    inner: R,
    sample_size: usize,
    buffer: Vec<u8>,
}
```

Ensures reads are sample-aligned.

## Usage

### Basic Resampling

```rust
use giztoy_audio::resampler::{Soxr, Format};

// 24kHz to 16kHz
let src_fmt = Format { sample_rate: 24000, stereo: false };
let dst_fmt = Format { sample_rate: 16000, stereo: false };

let mut resampler = Soxr::new(src_fmt, dst_fmt)?;

// Resample data
let output = resampler.process(&input_pcm)?;

// Flush at end of stream
let final_output = resampler.flush()?;
```

### Format Calculations

```rust
use giztoy_audio::resampler::Format;
use std::time::Duration;

let format = Format::L16Mono16K;

// Calculate buffer size for 100ms
let bytes = format.bytes_in_duration(Duration::from_millis(100));
// bytes = 3200

let samples = format.samples_in_duration(Duration::from_millis(100));
// samples = 1600

// Convert bytes to duration
let duration = format.duration(3200);
// duration = 100ms
```

### Custom Format

```rust
// 44.1kHz stereo (CD quality)
let cd_format = Format {
    sample_rate: 44100,
    stereo: true,
};

// 8kHz mono (telephone)
let phone_format = Format {
    sample_rate: 8000,
    stereo: false,
};
```

## FFI Bindings

```rust
// resampler/ffi.rs
#[repr(C)]
pub struct soxr {
    _private: [u8; 0],
}

extern "C" {
    pub fn soxr_create(
        input_rate: f64,
        output_rate: f64,
        num_channels: c_uint,
        error: *mut soxr_error,
        io_spec: *const soxr_io_spec,
        quality_spec: *const soxr_quality_spec,
        runtime_spec: *const soxr_runtime_spec,
    ) -> *mut soxr;
    
    pub fn soxr_process(
        soxr: *mut soxr,
        in_: soxr_in,
        ilen: usize,
        idone: *mut usize,
        out: soxr_out,
        olen: usize,
        odone: *mut usize,
    ) -> soxr_error;
    
    pub fn soxr_delete(soxr: *mut soxr);
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| API style | io.Reader wrapper | Direct process() method |
| Input source | Takes io.Reader | Takes byte slices |
| Channel convert | Built-in stereo↔mono | TBD |
| Error on close | CloseWithError | Drop trait |
| Format location | In resampler/ | In resampler/, re-exported by pcm/ |

## Thread Safety

- `Soxr` is NOT thread-safe
- Use separate instances for concurrent processing
