//! Ogg container format.
//!
//! This module implements the Ogg bitstream format as defined in RFC 3533.

mod page;
mod stream;
mod encoder;
mod sync;

pub use page::*;
pub use stream::*;
pub use encoder::*;
pub use sync::*;
