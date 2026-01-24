# Audio Codec - Rust Implementation

Crate: `giztoy-audio`

📚 [Rust Documentation](https://docs.rs/giztoy-audio) (module `codec`)

## opus Module

```rust
use giztoy_audio::codec::opus::{Encoder, Decoder, Application, Frame, TOC, FrameDuration};
```

### Types

#### Encoder

```rust
pub struct Encoder {
    sample_rate: i32,
    channels: i32,
    enc: *mut ffi::OpusEncoder,
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(sample_rate: i32, channels: i32, app: Application) -> Result<Self>` | Create encoder |
| `encode` | `fn encode(&mut self, pcm: &[i16], frame_size: i32) -> Result<Frame>` | Encode PCM |
| `set_bitrate` | `fn set_bitrate(&mut self, bitrate: i32) -> Result<()>` | Set target bitrate |
| `set_complexity` | `fn set_complexity(&mut self, complexity: i32) -> Result<()>` | Set CPU complexity |
| `frame_size_20ms` | `fn frame_size_20ms(&self) -> i32` | Get 20ms frame size |

**Application Enum:**

```rust
pub enum Application {
    VoIP,
    Audio,
    RestrictedLowdelay,
}
```

#### Decoder

```rust
pub struct Decoder {
    sample_rate: i32,
    channels: i32,
    dec: *mut ffi::OpusDecoder,
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(sample_rate: i32, channels: i32) -> Result<Self>` | Create decoder |
| `decode` | `fn decode(&mut self, frame: &Frame) -> Result<Vec<i16>>` | Decode to PCM |
| `decode_plc` | `fn decode_plc(&mut self, samples: i32) -> Result<Vec<i16>>` | Packet loss concealment |

#### Frame & TOC

```rust
pub type Frame = Vec<u8>;

pub struct TOC {
    pub config: u8,
    pub stereo: bool,
    pub frame_count: u8,
    pub duration: FrameDuration,
}

pub enum FrameDuration {
    Ms2_5,
    Ms5,
    Ms10,
    Ms20,
    Ms40,
    Ms60,
}
```

### Usage

```rust
use giztoy_audio::codec::opus::{Encoder, Decoder, Application};

// Create encoder
let mut encoder = Encoder::new(16000, 1, Application::VoIP)?;
encoder.set_bitrate(24000)?;

// Encode 20ms of audio
let frame_size = encoder.frame_size_20ms();
let pcm: Vec<i16> = vec![0; frame_size as usize];
let frame = encoder.encode(&pcm, frame_size)?;

// Create decoder
let mut decoder = Decoder::new(16000, 1)?;

// Decode
let pcm_out = decoder.decode(&frame)?;

// Handle packet loss
let plc_out = decoder.decode_plc(320)?;
```

---

## mp3 Module

```rust
use giztoy_audio::codec::mp3::{Encoder, Decoder};
```

### Encoder (LAME)

```rust
let mut encoder = Encoder::new(sample_rate, channels, bitrate)?;
let mp3_data = encoder.encode(&pcm_data)?;
```

### Decoder (minimp3)

```rust
let mut decoder = Decoder::new()?;
let pcm_data = decoder.decode(&mp3_data)?;
```

---

## ogg Module

```rust
use giztoy_audio::codec::ogg::{Encoder, Stream, Sync, Page};
```

### Stream

```rust
let mut stream = Stream::new(serial_no);

// Add packets
stream.packet_in(&packet);

// Get pages
while let Some(page) = stream.page_out() {
    // Write page
}
```

### Sync (for reading)

```rust
let mut sync = Sync::new();

// Feed data
sync.buffer(&data);

// Read pages
while let Some(page) = sync.page_out() {
    // Process page
}
```

---

## FFI Implementation

Each codec has an `ffi.rs` module with raw C bindings:

```rust
// codec/opus/ffi.rs
#[repr(C)]
pub struct OpusEncoder {
    _private: [u8; 0],
}

extern "C" {
    pub fn opus_encoder_create(
        fs: i32,
        channels: i32,
        application: i32,
        error: *mut i32,
    ) -> *mut OpusEncoder;
    
    pub fn opus_encode(
        enc: *mut OpusEncoder,
        pcm: *const i16,
        frame_size: i32,
        data: *mut u8,
        max_data_bytes: i32,
    ) -> i32;
    
    // ...
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| FFI approach | CGO with inline C | Separate ffi.rs module |
| Error handling | `error` return | `Result<T, E>` |
| Application type | int constants | enum |
| Memory management | GC + C.free | Drop trait |
| Frame type | `[]byte` | `Vec<u8>` |
