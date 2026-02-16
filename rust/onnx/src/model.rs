//! Model registry: register and load ONNX models by ID.

use std::collections::HashMap;
use std::sync::Mutex;

use once_cell::sync::Lazy;

use crate::error::OnnxError;
use crate::onnx::{Env, Session};

/// Identifies a built-in ONNX model.
pub struct ModelId;

impl ModelId {
    /// 3D-Speaker ERes2Net base model for speaker embedding extraction.
    /// Input: [1, T, 80] float32 (mel filterbank features)
    /// Output: [1, 512] float32 (speaker embedding)
    pub const SPEAKER_ERES2NET: &str = "speaker-eres2net";

    /// Silero VAD model for voice activity detection.
    pub const VAD_SILERO: &str = "vad-silero";

    /// Microsoft NSNet2 noise suppression model.
    pub const DENOISE_NSNET2: &str = "denoise-nsnet2";
}

/// Describes a registered model.
pub struct ModelInfo {
    pub id: String,
    pub data: &'static [u8],
}

pub(crate) static REGISTRY: Lazy<Mutex<HashMap<String, ModelInfo>>> =
    Lazy::new(|| Mutex::new(HashMap::new()));

/// Registers a model with the given ID and ONNX data.
pub fn register_model(id: &str, data: &'static [u8]) {
    let mut reg = REGISTRY.lock().unwrap();
    reg.insert(
        id.to_string(),
        ModelInfo {
            id: id.to_string(),
            data,
        },
    );
}

/// Loads a registered model by ID, returning a ready-to-use Session.
pub fn load_model(env: &Env, id: &str) -> Result<Session, OnnxError> {
    let reg = REGISTRY.lock().unwrap();
    let info = reg
        .get(id)
        .ok_or_else(|| OnnxError::ModelNotRegistered(id.to_string()))?;
    env.new_session(info.data)
}

/// Returns the IDs of all registered models.
pub fn list_models() -> Vec<String> {
    let reg = REGISTRY.lock().unwrap();
    reg.keys().cloned().collect()
}
