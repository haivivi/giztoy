# Audio OpusRT - Rust Implementation

Crate: `giztoy-audio`

📚 [Rust Documentation](https://docs.rs/giztoy-audio) (module `opusrt`)

## Types

### EpochMillis

```rust
pub type EpochMillis = i64;
```

**Functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `now` | `fn now() -> EpochMillis` | Current time as epoch millis |
| `from_duration` | `fn from_duration(d: Duration) -> EpochMillis` | Duration to millis |
| `to_duration` | `fn to_duration(e: EpochMillis) -> Duration` | Millis to Duration |

### Frame

```rust
pub type Frame = Vec<u8>;
```

**Extension Functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `frame_duration` | `fn frame_duration(frame: &Frame) -> Duration` | Frame duration from TOC |

### StampedFrame

```rust
pub struct StampedFrame {
    pub frame: Frame,
    pub timestamp: EpochMillis,
}

impl StampedFrame {
    pub fn from_bytes(data: &[u8]) -> Option<Self>;
    pub fn to_bytes(&self) -> Vec<u8>;
}
```

### Buffer

```rust
pub struct Buffer {
    duration: EpochMillis,
    // ... internal fields
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(duration: Duration) -> Self` | Create buffer |
| `append` | `fn append(&mut self, frame: Frame, stamp: EpochMillis) -> Result<()>` | Add frame |
| `frame` | `fn frame(&mut self) -> Result<(Option<Frame>, Duration)>` | Get next frame |
| `len` | `fn len(&self) -> usize` | Frame count |
| `buffered` | `fn buffered(&self) -> Duration` | Buffered duration |
| `reset` | `fn reset(&mut self)` | Clear buffer |

## Usage

### Basic Jitter Buffer

```rust
use giztoy_audio::opusrt::{Buffer, Frame, EpochMillis};
use std::time::Duration;

// Create buffer (2 minute capacity)
let mut buffer = Buffer::new(Duration::from_secs(120));

// Append frames
buffer.append(frame1, timestamp1)?;
buffer.append(frame2, timestamp2)?;

// Read in order
loop {
    match buffer.frame() {
        Ok((Some(frame), Duration::ZERO)) => {
            // Got a valid frame
            let pcm = decoder.decode(&frame)?;
            play(&pcm);
        }
        Ok((None, loss)) if loss > Duration::ZERO => {
            // Packet loss detected
            let plc_pcm = decoder.decode_plc(loss_to_samples(loss))?;
            play(&plc_pcm);
        }
        Err(_) => break, // Buffer empty
        _ => {}
    }
}
```

### Stamped Frame Handling

```rust
use giztoy_audio::opusrt::StampedFrame;

// Parse stamped frame
let stamped = StampedFrame::from_bytes(&network_data)?;
buffer.append(stamped.frame, stamped.timestamp)?;

// Create stamped frame
let stamped = StampedFrame {
    frame: opus_frame,
    timestamp: now(),
};
let bytes = stamped.to_bytes();
```

## Missing Features vs Go

| Feature | Go | Rust | Notes |
|---------|:--:|:----:|-------|
| Buffer | ✅ | ✅ | Basic jitter buffer |
| RealtimeBuffer | ✅ | ❌ | Real-time playback simulation |
| OggWriter | ✅ | ❌ | Write Opus to OGG |
| OggReader | ✅ | ❌ | Read Opus from OGG |
| RealtimeReader | ✅ | ❌ | Generic frame reader with pacing |

## Implementation Status

The Rust implementation provides:
- Basic jitter buffer with heap-based ordering
- Stamped frame parsing
- Timestamp utilities

**Missing:**
- RealtimeBuffer (background thread with real-time pacing)
- OGG container integration
- RealtimeReader wrapper

## Differences from Go

| Aspect | Go | Rust |
|--------|----|----- |
| EpochMillis | Custom type with methods | Type alias `i64` |
| Frame | `[]byte` with methods | `Vec<u8>` (no methods) |
| Error handling | Multiple error vars | Result with error types |
| Thread safety | Mutex-protected | TBD |
| OGG support | ✅ Full | ❌ Missing |
