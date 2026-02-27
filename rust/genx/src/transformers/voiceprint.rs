use std::sync::Arc;

use async_trait::async_trait;
use giztoy_voiceprint::{Detector, DetectorConfig, Hasher, SpeakerStatus, VoiceprintModel};
use tokio::sync::mpsc;

use crate::error::GenxError;
use crate::stream::Stream;
use crate::stream_utils::channel_stream;
use crate::transformer::Transformer;
use crate::types::MessageChunk;

#[derive(Debug, Clone)]
pub struct VoiceprintConfig {
    pub segment_duration_ms: usize,
    pub sample_rate: usize,
    pub detector_window_size: usize,
    pub detector_min_ratio: f32,
}

impl Default for VoiceprintConfig {
    fn default() -> Self {
        Self {
            segment_duration_ms: 400,
            sample_rate: 16_000,
            detector_window_size: 5,
            detector_min_ratio: 0.6,
        }
    }
}

pub struct VoiceprintTransformer {
    model: Arc<dyn VoiceprintModel>,
    hasher: Arc<Hasher>,
    config: VoiceprintConfig,
}

impl VoiceprintTransformer {
    pub fn new(model: Arc<dyn VoiceprintModel>, hasher: Arc<Hasher>, config: VoiceprintConfig) -> Self {
        Self {
            model,
            hasher,
            config,
        }
    }

    fn segment_bytes(&self) -> usize {
        self.config.sample_rate * 2 * self.config.segment_duration_ms / 1000
    }

    fn validate_config(&self) -> Result<(), GenxError> {
        if self.segment_bytes() == 0 {
            return Err(GenxError::Other(anyhow::anyhow!(
                "invalid voiceprint config: segment bytes is zero (sample_rate={}, segment_duration_ms={})",
                self.config.sample_rate,
                self.config.segment_duration_ms
            )));
        }
        Ok(())
    }

    fn is_pcm_mime(mime: &str) -> bool {
        mime == "audio/pcm" || mime.starts_with("audio/pcm;")
    }

    fn process_segment(&self, detector: &mut Detector, pcm: &[u8], current_label: &str) -> String {
        let embedding = match self.model.extract(pcm) {
            Ok(v) => v,
            Err(_) => return current_label.to_string(),
        };

        let hash = self.hasher.hash(&embedding);
        let Some(result) = detector.feed(&hash) else {
            return current_label.to_string();
        };

        match result.status {
            SpeakerStatus::Single | SpeakerStatus::Overlap => result.speaker,
            SpeakerStatus::Unknown => current_label.to_string(),
        }
    }

    fn annotate_label(chunk: &mut MessageChunk, label: &str) {
        if label.is_empty() {
            return;
        }
        if chunk.ctrl.is_none() {
            chunk.ctrl = Some(Default::default());
        }
        if let Some(ctrl) = chunk.ctrl.as_mut() {
            ctrl.label = label.to_string();
        }
    }
}

#[async_trait]
impl Transformer for VoiceprintTransformer {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        self.validate_config()?;

        let (tx, rx) = mpsc::channel(128);

        let model = Arc::clone(&self.model);
        let hasher = Arc::clone(&self.hasher);
        let config = self.config.clone();

        tokio::spawn(async move {
            let this = VoiceprintTransformer::new(model, hasher, config.clone());
            let mut detector = Detector::with_config(DetectorConfig {
                window_size: config.detector_window_size,
                min_ratio: config.detector_min_ratio,
            });
            let mut input = input;
            let mut pcm_accum = Vec::<u8>::new();
            let mut last_label = String::new();
            let seg_bytes = this.segment_bytes();

            loop {
                match input.next().await {
                    Ok(Some(mut chunk)) => {
                        if chunk.is_end_of_stream() {
                            if let Some(blob) = chunk.part.as_ref().and_then(|p| p.as_blob())
                                && VoiceprintTransformer::is_pcm_mime(&blob.mime_type)
                            {
                                if !pcm_accum.is_empty() {
                                    last_label =
                                        this.process_segment(&mut detector, &pcm_accum, &last_label);
                                    pcm_accum.clear();
                                }
                                VoiceprintTransformer::annotate_label(&mut chunk, &last_label);
                                if tx.send(Ok(chunk)).await.is_err() {
                                    return;
                                }
                                continue;
                            }

                            if tx.send(Ok(chunk)).await.is_err() {
                                return;
                            }
                            continue;
                        }

                        if let Some(blob) = chunk.part.as_ref().and_then(|p| p.as_blob())
                            && VoiceprintTransformer::is_pcm_mime(&blob.mime_type)
                        {
                            pcm_accum.extend_from_slice(&blob.data);
                            let mut processed = 0usize;
                            while pcm_accum.len().saturating_sub(processed) >= seg_bytes {
                                last_label = this.process_segment(
                                    &mut detector,
                                    &pcm_accum[processed..processed + seg_bytes],
                                    &last_label,
                                );
                                processed += seg_bytes;
                            }

                            if processed > 0 {
                                pcm_accum.copy_within(processed.., 0);
                                pcm_accum.truncate(pcm_accum.len() - processed);
                            }

                            VoiceprintTransformer::annotate_label(&mut chunk, &last_label);
                            if tx.send(Ok(chunk)).await.is_err() {
                                return;
                            }
                            continue;
                        }

                        if tx.send(Ok(chunk)).await.is_err() {
                            return;
                        }
                    }
                    Ok(None) => {
                        if !pcm_accum.is_empty() {
                            let _ = this.process_segment(&mut detector, &pcm_accum, &last_label);
                        }
                        return;
                    }
                    Err(e) => {
                        let _ = tx.send(Err(e.to_string())).await;
                        return;
                    }
                }
            }
        });

        Ok(channel_stream(rx))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use giztoy_voiceprint::VoiceprintError;

    use crate::types::Role;

    struct MockModel {
        dimension: usize,
        fail: bool,
    }

    impl VoiceprintModel for MockModel {
        fn extract(&self, _audio: &[u8]) -> Result<Vec<f32>, VoiceprintError> {
            if self.fail {
                return Err(VoiceprintError::Model("mock-fail".into()));
            }
            Ok(vec![1.0; self.dimension])
        }

        fn dimension(&self) -> usize {
            self.dimension
        }
    }

    fn make_hasher(dim: usize) -> Arc<Hasher> {
        let mut planes = Vec::new();
        for _ in 0..16 {
            planes.push(vec![1.0; dim]);
        }
        Arc::new(Hasher::from_planes(dim, 16, planes))
    }

    fn make_input(chunks: Vec<MessageChunk>) -> Box<dyn Stream> {
        let (tx, rx) = mpsc::channel(64);
        tokio::spawn(async move {
            for chunk in chunks {
                if tx.send(Ok(chunk)).await.is_err() {
                    break;
                }
            }
        });
        channel_stream(rx)
    }

    fn make_transformer(model: Arc<dyn VoiceprintModel>) -> VoiceprintTransformer {
        VoiceprintTransformer::new(
            model,
            make_hasher(8),
            VoiceprintConfig {
                segment_duration_ms: 200,
                sample_rate: 16_000,
                detector_window_size: 2,
                detector_min_ratio: 0.5,
            },
        )
    }

    #[tokio::test]
    async fn t_voiceprint_passthrough_non_pcm() {
        let t = make_transformer(Arc::new(MockModel {
            dimension: 8,
            fail: false,
        }));

        let input = make_input(vec![
            MessageChunk::text(Role::User, "hi"),
            MessageChunk::blob(Role::User, "audio/mp3", vec![1, 2, 3]),
        ]);

        let mut out = t.transform("", input).await.unwrap();
        let c1 = out.next().await.unwrap().unwrap();
        let c2 = out.next().await.unwrap().unwrap();

        assert_eq!(c1.part.as_ref().and_then(|p| p.as_text()), Some("hi"));
        assert_eq!(c2.ctrl.as_ref().map(|c| c.label.as_str()), None);
    }

    #[tokio::test]
    async fn t_voiceprint_normal_labeling() {
        let t = make_transformer(Arc::new(MockModel {
            dimension: 8,
            fail: false,
        }));

        let pcm = vec![0u8; 16_000 * 2 / 2];
        let input = make_input(vec![MessageChunk::blob(Role::User, "audio/pcm", pcm)]);

        let mut out = t.transform("", input).await.unwrap();
        let c = out.next().await.unwrap().unwrap();
        assert!(!c.ctrl.as_ref().map(|v| v.label.clone()).unwrap_or_default().is_empty());
    }

    #[tokio::test]
    async fn t_voiceprint_pcm_eos_handling() {
        let t = make_transformer(Arc::new(MockModel {
            dimension: 8,
            fail: false,
        }));

        let mut eos = MessageChunk::new_end_of_stream("audio/pcm");
        eos.role = Role::Model;
        eos.name = Some("n1".into());

        let input = make_input(vec![
            MessageChunk::blob(Role::Model, "audio/pcm", vec![0u8; 16_000]),
            eos,
        ]);

        let mut out = t.transform("", input).await.unwrap();
        let _ = out.next().await.unwrap().unwrap();
        let eos_out = out.next().await.unwrap().unwrap();

        assert!(eos_out.is_end_of_stream());
        assert_eq!(eos_out.role, Role::Model);
        assert_eq!(eos_out.name.as_deref(), Some("n1"));
    }

    #[tokio::test]
    async fn t_voiceprint_model_error_continue() {
        let t = make_transformer(Arc::new(MockModel {
            dimension: 8,
            fail: true,
        }));
        let input = make_input(vec![MessageChunk::blob(
            Role::User,
            "audio/pcm",
            vec![0u8; 16_000],
        )]);
        let mut out = t.transform("", input).await.unwrap();
        let c = out.next().await.unwrap().unwrap();
        assert!(c.ctrl.as_ref().map(|v| v.label.is_empty()).unwrap_or(true));
    }

    #[tokio::test]
    async fn t_voiceprint_reject_zero_segment_size() {
        let t = VoiceprintTransformer::new(
            Arc::new(MockModel {
                dimension: 8,
                fail: false,
            }),
            make_hasher(8),
            VoiceprintConfig {
                segment_duration_ms: 0,
                sample_rate: 16_000,
                detector_window_size: 2,
                detector_min_ratio: 0.5,
            },
        );

        let err = match t.transform("", make_input(vec![])).await {
            Ok(_) => panic!("expected invalid config error"),
            Err(e) => e,
        };
        assert!(err.to_string().contains("segment bytes is zero"));
    }
}
