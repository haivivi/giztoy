//! Ogg container format.
//!
//! This module implements the Ogg bitstream format as defined in RFC 3533.

mod page;
mod stream;
mod encoder;
pub mod opus_reader;
pub mod opus_writer;
mod sync;

pub use page::*;
pub use stream::*;
pub use encoder::*;
pub use opus_reader::{read_opus_packets, OpusPacketReader, OpusPacketIter};
pub use opus_writer::{OpusWriter, OpusPacket};
pub use sync::*;
