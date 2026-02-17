//! Embedded ONNX model files.

use crate::model::{register_model, ModelId};

static SPEAKER_ERES2NET: &[u8] = include_bytes!("speaker_eres2net.onnx");
static VAD_SILERO: &[u8] = include_bytes!("vad_silero.onnx");
static DENOISE_NSNET2: &[u8] = include_bytes!("denoise_nsnet2.onnx");

/// Registers all built-in embedded ONNX models.
pub fn register_embedded_models() {
    register_model(ModelId::SPEAKER_ERES2NET, SPEAKER_ERES2NET);
    register_model(ModelId::VAD_SILERO, VAD_SILERO);
    register_model(ModelId::DENOISE_NSNET2, DENOISE_NSNET2);
}
