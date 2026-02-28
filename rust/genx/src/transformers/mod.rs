//! Stream transformers for audio and text processing.
//!
//! # Supported Backends
//!
//! - MiniMax TTS (`minimax-tts` feature)
//! - Doubao TTS Seed V2 (`doubao-tts` feature)
//! - Doubao TTS ICL V2 (`doubao-tts` feature)
//! - Doubao ASR SAUC (`doubao-asr` feature)
//! - Doubao Realtime (`doubao-realtime` feature)
//! - DashScope Realtime (`dashscope-realtime` feature)
//! - MP3→OGG codec
//! - Voiceprint
//!
//! # Lifecycle
//!
//! All transformers follow the genx::Transformer lifecycle contract:
//! - `transform()` uses the async context only for initialization
//! - Background tasks exit when `input.next()` returns `None` or `Err`
//! - To cancel, drop the input Stream

mod mux;
mod tts_core;
mod minimax_tts;
mod doubao_tts_seed_v2;
mod doubao_tts_icl_v2;
mod doubao_asr_sauc;
mod doubao_realtime;
mod dashscope_realtime;
mod codec_mp3_to_ogg;
mod voiceprint;

pub use mux::*;
pub use minimax_tts::MinimaxTtsTransformer;
pub use doubao_tts_seed_v2::DoubaoTtsSeedV2Transformer;
pub use doubao_tts_icl_v2::DoubaoTtsIclV2Transformer;
pub use doubao_asr_sauc::DoubaoAsrSaucTransformer;
pub use doubao_realtime::*;
pub use dashscope_realtime::*;
pub use codec_mp3_to_ogg::*;
pub use voiceprint::*;

use std::sync::Arc;

use crate::error::GenxError;
use crate::transformer::Transformer;

/// Register Doubao realtime transformer into mux using canonical route name.
pub fn register_doubao_realtime(
    mux: &mut TransformerMux,
    transformer: Arc<dyn Transformer>,
) -> Result<(), GenxError> {
    mux.handle("doubao/realtime", transformer)
}

/// Register DashScope realtime transformer into mux using canonical route name.
pub fn register_dashscope_realtime(
    mux: &mut TransformerMux,
    transformer: Arc<dyn Transformer>,
) -> Result<(), GenxError> {
    mux.handle("dashscope/realtime", transformer)
}
