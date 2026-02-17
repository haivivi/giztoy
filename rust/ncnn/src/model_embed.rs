//! Embedded ncnn model files.
//!
//! Model files are embedded at compile time via Bazel genrule + include_bytes!.
//! Call [`register_embedded_models`] to register all built-in models.

use crate::model::{register_model, ModelId};

// Embedded model data — these files are copied into the build sandbox
// by the Bazel genrule "embed_models" in BUILD.bazel.
static SPEAKER_ERES2NET_PARAM: &[u8] = include_bytes!("speaker_eres2net.ncnn.param");
static SPEAKER_ERES2NET_BIN: &[u8] = include_bytes!("speaker_eres2net.ncnn.bin");

static VAD_SILERO_PARAM: &[u8] = include_bytes!("vad_silero.ncnn.param");
static VAD_SILERO_BIN: &[u8] = include_bytes!("vad_silero.ncnn.bin");

static DENOISE_NSNET2_PARAM: &[u8] = include_bytes!("denoise_nsnet2.ncnn.param");
static DENOISE_NSNET2_BIN: &[u8] = include_bytes!("denoise_nsnet2.ncnn.bin");

/// Registers all built-in embedded models.
/// Call this once at startup to make the models available via [`crate::load_model`].
pub fn register_embedded_models() {
    register_model(ModelId::SPEAKER_ERES2NET, SPEAKER_ERES2NET_PARAM, SPEAKER_ERES2NET_BIN);
    register_model(ModelId::VAD_SILERO, VAD_SILERO_PARAM, VAD_SILERO_BIN);
    register_model(ModelId::DENOISE_NSNET2, DENOISE_NSNET2_PARAM, DENOISE_NSNET2_BIN);
}
