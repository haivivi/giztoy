//! [`VoiceprintModel`] implementation using the ncnn inference engine.

use std::sync::RwLock;

use giztoy_ncnn::{Mat, NcnnOption, Net};

use crate::error::VoiceprintError;
use crate::fbank::{cmvn, compute_fbank, l2_normalize, FbankConfig};
use crate::model::VoiceprintModel;

/// Number of fbank frames per inference segment.
/// 300 frames = 3 seconds at 10ms hop.
const SEG_FRAMES: usize = 300;

/// Hop between segments for averaging.
const HOP_FRAMES: usize = 150;

/// [`VoiceprintModel`] implementation using the ncnn inference engine.
///
/// Wraps a [`Net`] and handles the full pipeline from PCM audio to
/// speaker embedding vector.
///
/// # Pipeline
///
/// 1. PCM16 audio -> [`compute_fbank`] -> mel filterbank features
/// 2. Fbank features -> [`cmvn`] normalization
/// 3. Segment-based ncnn inference (300-frame windows, 150-frame hop)
/// 4. Average segment embeddings + L2 normalize
///
/// # Thread Safety
///
/// NCNNModel is safe for concurrent use. The ncnn Net is loaded once
/// and shared; each `extract` call creates its own Extractor.
pub struct NCNNModel {
    inner: RwLock<NCNNModelInner>,
}

struct NCNNModelInner {
    net: Option<Net>,
    dim: usize,
    fbank_cfg: FbankConfig,
    input_name: String,
    output_name: String,
    closed: bool,
}

/// Configuration for [`NCNNModel`].
pub struct NCNNModelConfig {
    /// Expected embedding dimension (default: 512).
    pub dim: usize,
    /// Filterbank configuration.
    pub fbank_cfg: FbankConfig,
    /// ncnn input blob name (default: "in0").
    pub input_name: String,
    /// ncnn output blob name (default: "out0").
    pub output_name: String,
}

impl Default for NCNNModelConfig {
    fn default() -> Self {
        Self {
            dim: 512,
            fbank_cfg: FbankConfig::default(),
            input_name: "in0".to_string(),
            output_name: "out0".to_string(),
        }
    }
}

impl NCNNModel {
    /// Creates a new NCNNModel from in-memory .param and .bin data.
    /// FP16 is disabled by default for numerical safety.
    pub fn from_memory(
        param_data: &[u8],
        bin_data: &[u8],
        cfg: NCNNModelConfig,
    ) -> Result<Self, VoiceprintError> {
        if param_data.is_empty() || bin_data.is_empty() {
            return Err(VoiceprintError::Model("empty model data".into()));
        }

        let mut opt = NcnnOption::new().map_err(|e| VoiceprintError::Model(e.to_string()))?;
        opt.set_fp16(false);
        let net = Net::from_memory(param_data, bin_data, Some(&opt))
            .map_err(|e| VoiceprintError::Model(e.to_string()))?;

        Ok(Self {
            inner: RwLock::new(NCNNModelInner {
                net: Some(net),
                dim: cfg.dim,
                fbank_cfg: cfg.fbank_cfg,
                input_name: cfg.input_name,
                output_name: cfg.output_name,
                closed: false,
            }),
        })
    }

    /// Creates a new NCNNModel from a pre-loaded Net.
    pub fn from_net(net: Net, cfg: NCNNModelConfig) -> Self {
        Self {
            inner: RwLock::new(NCNNModelInner {
                net: Some(net),
                dim: cfg.dim,
                fbank_cfg: cfg.fbank_cfg,
                input_name: cfg.input_name,
                output_name: cfg.output_name,
                closed: false,
            }),
        }
    }

    /// Closes the model and releases resources.
    pub fn close(&self) {
        let mut inner = self.inner.write().unwrap();
        if !inner.closed {
            inner.closed = true;
            inner.net = None;
        }
    }
}

impl VoiceprintModel for NCNNModel {
    fn extract(&self, audio: &[u8]) -> Result<Vec<f32>, VoiceprintError> {
        let inner = self.inner.read().unwrap();
        if inner.closed {
            return Err(VoiceprintError::Closed);
        }
        let net = inner.net.as_ref().ok_or(VoiceprintError::Closed)?;

        // Step 1: Compute fbank features.
        let mut features =
            compute_fbank(audio, &inner.fbank_cfg).ok_or(VoiceprintError::AudioTooShort {
                min_bytes: inner.fbank_cfg.frame_length * 2,
                got_bytes: audio.len(),
            })?;

        if features.is_empty() {
            return Err(VoiceprintError::AudioTooShort {
                min_bytes: inner.fbank_cfg.frame_length * 2,
                got_bytes: audio.len(),
            });
        }

        // Step 2: CMVN normalization.
        cmvn(&mut features);

        // Step 3: Segment-based extraction with averaging.
        let num_frames = features.len();
        if num_frames <= SEG_FRAMES {
            let mut emb = extract_segment(
                net,
                &features,
                &inner.input_name,
                &inner.output_name,
                inner.dim,
            )?;
            l2_normalize(&mut emb);
            return Ok(emb);
        }

        // Long audio: sliding window, average all segment embeddings.
        let mut embeddings: Vec<Vec<f32>> = Vec::new();
        let mut last_start = 0;
        let mut start = 0;
        while start + SEG_FRAMES <= num_frames {
            if let Ok(mut emb) = extract_segment(
                net,
                &features[start..start + SEG_FRAMES],
                &inner.input_name,
                &inner.output_name,
                inner.dim,
            ) {
                l2_normalize(&mut emb);
                embeddings.push(emb);
            }
            last_start = start;
            start += HOP_FRAMES;
        }

        // Ensure the last segment covers the end of the audio.
        let tail = num_frames - SEG_FRAMES;
        if tail > last_start {
            if let Ok(mut emb) = extract_segment(
                net,
                &features[tail..],
                &inner.input_name,
                &inner.output_name,
                inner.dim,
            ) {
                l2_normalize(&mut emb);
                embeddings.push(emb);
            }
        }

        if embeddings.is_empty() {
            return Err(VoiceprintError::Model("all segments failed".into()));
        }

        // Average all segment embeddings.
        let mut avg = vec![0.0f32; inner.dim];
        for emb in &embeddings {
            for (i, &v) in emb.iter().enumerate() {
                avg[i] += v;
            }
        }
        let n = embeddings.len() as f32;
        for v in &mut avg {
            *v /= n;
        }
        l2_normalize(&mut avg);
        Ok(avg)
    }

    fn dimension(&self) -> usize {
        self.inner.read().unwrap().dim
    }
}

/// Runs ncnn inference on a single fbank segment.
fn extract_segment(
    net: &Net,
    features: &[Vec<f32>],
    input_name: &str,
    output_name: &str,
    dim: usize,
) -> Result<Vec<f32>, VoiceprintError> {
    let num_frames = features.len();
    let num_mels = features[0].len();

    let mut flat_data = vec![0.0f32; num_frames * num_mels];
    for (t, frame) in features.iter().enumerate() {
        flat_data[t * num_mels..t * num_mels + num_mels].copy_from_slice(frame);
    }

    let input = Mat::new_2d(num_mels as i32, num_frames as i32, &flat_data)
        .map_err(|e| VoiceprintError::Model(format!("create input mat: {e}")))?;

    let mut ex = net
        .extractor()
        .map_err(|e| VoiceprintError::Model(format!("create extractor: {e}")))?;

    ex.set_input(input_name, &input)
        .map_err(|e| VoiceprintError::Model(e.to_string()))?;

    let output = ex
        .extract(output_name)
        .map_err(|e| VoiceprintError::Model(e.to_string()))?;

    let data = output
        .float_data()
        .ok_or_else(|| VoiceprintError::Model("ncnn output data is nil".into()))?;

    let n = data.len().min(dim);
    let mut embedding = vec![0.0f32; dim];
    embedding[..n].copy_from_slice(&data[..n]);
    Ok(embedding)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ncnn_model_dimension() {
        giztoy_ncnn::register_embedded_models();
        let net = giztoy_ncnn::load_model(giztoy_ncnn::ModelId::SPEAKER_ERES2NET).unwrap();
        let model = NCNNModel::from_net(net, NCNNModelConfig::default());
        assert_eq!(model.dimension(), 512);
    }

    #[test]
    fn ncnn_model_extract_short_audio() {
        giztoy_ncnn::register_embedded_models();
        let net = giztoy_ncnn::load_model(giztoy_ncnn::ModelId::SPEAKER_ERES2NET).unwrap();
        let model = NCNNModel::from_net(net, NCNNModelConfig::default());

        // 1 second of silence (16000 samples = 32000 bytes).
        let audio = vec![0u8; 32000];
        let emb = model.extract(&audio).unwrap();
        assert_eq!(emb.len(), 512);

        // Should be L2-normalized (unit length).
        let norm: f64 = emb
            .iter()
            .map(|&x| (x as f64) * (x as f64))
            .sum::<f64>()
            .sqrt();
        assert!(
            (norm - 1.0).abs() < 1e-4,
            "embedding should be unit length, got {norm}"
        );
    }

    #[test]
    fn ncnn_model_too_short_audio() {
        giztoy_ncnn::register_embedded_models();
        let net = giztoy_ncnn::load_model(giztoy_ncnn::ModelId::SPEAKER_ERES2NET).unwrap();
        let model = NCNNModel::from_net(net, NCNNModelConfig::default());

        // 100 bytes = 50 samples, too short for a frame.
        let audio = vec![0u8; 100];
        assert!(model.extract(&audio).is_err());
    }

    #[test]
    fn ncnn_model_close() {
        giztoy_ncnn::register_embedded_models();
        let net = giztoy_ncnn::load_model(giztoy_ncnn::ModelId::SPEAKER_ERES2NET).unwrap();
        let model = NCNNModel::from_net(net, NCNNModelConfig::default());
        model.close();
        assert!(model.extract(&vec![0u8; 32000]).is_err());
    }
}
