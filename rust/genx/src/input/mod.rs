//! Input processing for real-time audio streams.

use thiserror::Error;

pub mod jitter_buffer;
pub mod opus;
pub mod opus_ogg_stream;
pub mod opus_stamped_stream;
pub mod opus_stream;

pub use jitter_buffer::{JitterBuffer, Timestamped};
pub use opus::{EpochMillis, OpusFrame, StampedFrame, make_stamped, parse_stamped};
pub use opus_ogg_stream::{OpusOggStream, is_opus_header};
pub use opus_stamped_stream::{
    RealtimeConfig, StampedOpusReader, StampedOpusStats, StampedOpusStream,
};
pub use opus_stream::{OpusReader, OpusStream};

/// 输入层错误类型。
#[derive(Debug, Error)]
pub enum InputError {
    #[error("invalid stamped frame: {0}")]
    InvalidStampedFrame(String),

    #[error("truncated frame data")]
    TruncatedFrame,

    #[error("unsupported version: {0}")]
    UnsupportedVersion(u8),

    #[error("ogg decode error: {0}")]
    OggDecodeError(String),

    #[error("stream closed")]
    StreamClosed,
}
