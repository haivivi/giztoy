//! Conversation segmentation — compressing messages into structured segments
//! with entity and relation extraction.

mod genx_impl;
mod mux;
mod prompt;
pub(crate) mod types;

use std::sync::{Arc, OnceLock, RwLock};

use crate::error::GenxError;

pub use genx_impl::GenXSegmentor;
pub use mux::SegmentorMux;
pub use types::*;

fn default_mux() -> &'static RwLock<SegmentorMux> {
    static DEFAULT_MUX: OnceLock<RwLock<SegmentorMux>> = OnceLock::new();
    DEFAULT_MUX.get_or_init(|| RwLock::new(SegmentorMux::new()))
}

/// Register a segmentor in the global default mux.
pub fn handle(pattern: impl Into<String>, s: Arc<dyn Segmentor>) -> Result<(), GenxError> {
    let mut mux = default_mux()
        .write()
        .map_err(|_| GenxError::Other(anyhow::anyhow!("segmentors: default mux lock poisoned")))?;
    mux.handle(pattern, s)
}

/// Process with the global default mux.
pub async fn process(pattern: &str, input: SegmentorInput) -> Result<SegmentorResult, GenxError> {
    let s = {
        let mux = default_mux()
            .read()
            .map_err(|_| GenxError::Other(anyhow::anyhow!("segmentors: default mux lock poisoned")))?;
        mux.get_arc(pattern)?
    };
    s.process(input).await
}
