//! Entity profile management — evolving schemas and updating profiles
//! based on segmentor output and conversation context.

mod genx_impl;
mod mux;
mod prompt;
mod types;

pub use genx_impl::GenXProfiler;
pub use mux::ProfilerMux;
pub use types::*;
