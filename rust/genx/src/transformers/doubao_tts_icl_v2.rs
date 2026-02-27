use std::sync::Arc;

use async_trait::async_trait;
use giztoy_doubaospeech as doubaospeech;

use crate::error::GenxError;
use crate::stream::Stream;
use crate::transformer::Transformer;

use super::doubao_tts_seed_v2::DoubaoTtsV2Provider;
use super::tts_core::{spawn_tts_transform_loop, TtsProvider};

pub struct DoubaoTtsIclV2Transformer {
    provider: Arc<dyn TtsProvider>,
}

impl DoubaoTtsIclV2Transformer {
    pub fn new(client: Arc<doubaospeech::Client>, speaker: impl Into<String>) -> Self {
        Self {
            provider: Arc::new(DoubaoTtsV2Provider {
                client,
                speaker: speaker.into(),
                resource_id: doubaospeech::RESOURCE_VOICE_CLONE_V2.to_string(),
                format: "ogg_opus".to_string(),
                sample_rate: Some(24000),
                bit_rate: None,
                speed_ratio: Some(1.0),
                volume_ratio: Some(1.0),
                pitch_ratio: Some(1.0),
                emotion: None,
                language: None,
            }),
        }
    }

    pub(crate) fn new_with_provider(provider: Arc<dyn TtsProvider>) -> Self {
        Self { provider }
    }
}

#[async_trait]
impl Transformer for DoubaoTtsIclV2Transformer {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        Ok(spawn_tts_transform_loop(Arc::clone(&self.provider), input))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::transformers::tts_core::AudioEmitter;
    use crate::types::{MessageChunk, Role};
    use tokio::sync::Mutex;

    struct MockProvider {
        responses: Mutex<Vec<Result<Vec<Vec<u8>>, GenxError>>>,
    }

    #[async_trait]
    impl TtsProvider for MockProvider {
        fn mime_type(&self) -> &str {
            "audio/ogg"
        }

        async fn synthesize_stream(
            &self,
            _text: &str,
            emitter: &mut dyn AudioEmitter,
        ) -> Result<(), GenxError> {
            match self.responses.lock().await.remove(0) {
                Ok(chunks) => {
                    for audio in chunks {
                        emitter.emit(audio).await?;
                    }
                    Ok(())
                }
                Err(e) => Err(e),
            }
        }
    }

    fn input_stream(chunks: Vec<MessageChunk>) -> Box<dyn Stream> {
        let builder = crate::stream::StreamBuilder::with_tools(16, vec![]);
        builder.add(&chunks).expect("add chunks");
        builder.done(crate::error::Usage::default()).expect("done");
        Box::new(builder.stream())
    }

    #[tokio::test]
    async fn tts_error_chain_is_propagated() {
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            responses: Mutex::new(vec![Err(GenxError::Other(anyhow::anyhow!(
                "icl provider error"
            )))]),
        });
        let t = DoubaoTtsIclV2Transformer::new_with_provider(provider);

        let mut out = t
            .transform(
                "doubao/icl-v2",
                input_stream(vec![
                    MessageChunk::text(Role::Model, "test"),
                    MessageChunk::new_text_end_of_stream(),
                ]),
            )
            .await
            .expect("transform");

        let _ = out.next().await.expect("bos").expect("bos chunk");
        let err = out.next().await.expect_err("provider error");
        assert!(err.to_string().contains("icl provider error"));
    }
}
