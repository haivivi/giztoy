//! Conversation segmentation — compressing messages into structured segments
//! with entity and relation extraction.

mod genx_impl;
mod mux;
mod prompt;
pub(crate) mod types;

pub use genx_impl::GenXSegmentor;
pub use mux::SegmentorMux;
pub use types::*;
