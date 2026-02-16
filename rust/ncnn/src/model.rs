//! Model registry: register and load ncnn models by ID.

use std::collections::HashMap;
use std::sync::Mutex;

use once_cell::sync::Lazy;

use crate::error::NcnnError;
use crate::ncnn::{NcnnOption, Net};

/// Identifies a built-in ncnn model.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct ModelId(pub String);

impl ModelId {
    /// 3D-Speaker ERes2Net base model for speaker embedding extraction.
    /// Input: [T, 80] float32 (mel filterbank features)
    /// Output: [512] float32 (speaker embedding)
    pub const SPEAKER_ERES2NET: &str = "speaker-eres2net";

    /// Silero VAD model for voice activity detection.
    /// Input: [batch, sequence] float32 (audio samples)
    /// Output: [batch, 1] float32 (speech probability)
    pub const VAD_SILERO: &str = "vad-silero";

    /// Microsoft NSNet2 noise suppression model.
    /// Operates frame-by-frame on log-power spectrum features.
    pub const DENOISE_NSNET2: &str = "denoise-nsnet2";
}

/// Describes a registered model.
pub struct ModelInfo {
    pub id: String,
    pub param_data: &'static [u8],
    pub bin_data: &'static [u8],
}

static REGISTRY: Lazy<Mutex<HashMap<String, ModelInfo>>> = Lazy::new(|| Mutex::new(HashMap::new()));

/// Registers a model with the given ID and data.
/// Registering the same ID twice replaces the previous registration.
pub fn register_model(id: &str, param_data: &'static [u8], bin_data: &'static [u8]) {
    let mut reg = REGISTRY.lock().unwrap();
    reg.insert(
        id.to_string(),
        ModelInfo {
            id: id.to_string(),
            param_data,
            bin_data,
        },
    );
}

/// Loads a registered model by ID, returning a ready-to-use Net.
/// FP16 is disabled by default for numerical safety.
pub fn load_model(id: &str) -> Result<Net, NcnnError> {
    let reg = REGISTRY.lock().unwrap();
    let info = reg
        .get(id)
        .ok_or_else(|| NcnnError::ModelNotRegistered(id.to_string()))?;

    let mut opt = NcnnOption::new()?;
    opt.set_fp16(false);
    Net::from_memory(info.param_data, info.bin_data, Some(&opt))
}

/// Returns the IDs of all registered models.
pub fn list_models() -> Vec<String> {
    let reg = REGISTRY.lock().unwrap();
    reg.keys().cloned().collect()
}

/// Returns true if the model is registered.
pub fn is_registered(id: &str) -> bool {
    let reg = REGISTRY.lock().unwrap();
    reg.contains_key(id)
}
