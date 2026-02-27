//! Entity profile management — evolving schemas and updating profiles
//! based on segmentor output and conversation context.

mod genx_impl;
mod mux;
mod prompt;
mod types;

use std::sync::{Arc, OnceLock, RwLock};

use crate::error::GenxError;

pub use genx_impl::GenXProfiler;
pub use mux::ProfilerMux;
pub use types::*;

fn default_mux() -> &'static RwLock<ProfilerMux> {
    static DEFAULT_MUX: OnceLock<RwLock<ProfilerMux>> = OnceLock::new();
    DEFAULT_MUX.get_or_init(|| RwLock::new(ProfilerMux::new()))
}

/// Register a profiler in the global default mux.
pub fn handle(pattern: impl Into<String>, p: Arc<dyn Profiler>) -> Result<(), GenxError> {
    let mut mux = default_mux()
        .write()
        .map_err(|_| GenxError::Other(anyhow::anyhow!("profilers: default mux lock poisoned")))?;
    mux.handle(pattern, p)
}

/// Process with the global default mux.
pub async fn process(pattern: &str, input: ProfilerInput) -> Result<ProfilerResult, GenxError> {
    let p = {
        let mux = default_mux()
            .read()
            .map_err(|_| GenxError::Other(anyhow::anyhow!("profilers: default mux lock poisoned")))?;
        mux.get_arc(pattern)?
    };
    p.process(input).await
}
