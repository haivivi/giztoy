use std::sync::Arc;

use async_trait::async_trait;
use futures::{pin_mut, StreamExt};
use giztoy_doubaospeech as doubaospeech;

use crate::error::GenxError;
use crate::stream::Stream;
use crate::transformer::Transformer;

use super::tts_core::{spawn_tts_transform_loop, AudioEmitter, TtsProvider};

pub struct DoubaoTtsSeedV2Transformer {
    provider: Arc<dyn TtsProvider>,
}

impl DoubaoTtsSeedV2Transformer {
    pub fn new(client: Arc<doubaospeech::Client>, speaker: impl Into<String>) -> Self {
        Self {
            provider: Arc::new(DoubaoTtsV2Provider {
                client,
                speaker: speaker.into(),
                resource_id: doubaospeech::RESOURCE_TTS_V2.to_string(),
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
impl Transformer for DoubaoTtsSeedV2Transformer {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        Ok(spawn_tts_transform_loop(Arc::clone(&self.provider), input))
    }
}

pub(crate) struct DoubaoTtsV2Provider {
    pub(crate) client: Arc<doubaospeech::Client>,
    pub(crate) speaker: String,
    pub(crate) resource_id: String,
    pub(crate) format: String,
    pub(crate) sample_rate: Option<i32>,
    pub(crate) bit_rate: Option<i32>,
    pub(crate) speed_ratio: Option<f64>,
    pub(crate) volume_ratio: Option<f64>,
    pub(crate) pitch_ratio: Option<f64>,
    pub(crate) emotion: Option<String>,
    pub(crate) language: Option<String>,
}

#[async_trait]
impl TtsProvider for DoubaoTtsV2Provider {
    fn mime_type(&self) -> &str {
        match self.format.as_str() {
            "mp3" => "audio/mpeg",
            "pcm" => "audio/pcm",
            "ogg_opus" => "audio/ogg",
            _ => "audio/ogg",
        }
    }

    async fn synthesize_stream(
        &self,
        text: &str,
        emitter: &mut dyn AudioEmitter,
    ) -> Result<(), GenxError> {
        let req = doubaospeech::TtsV2Request {
            text: text.to_string(),
            speaker: self.speaker.clone(),
            resource_id: Some(self.resource_id.clone()),
            format: Some(self.format.clone()),
            sample_rate: self.sample_rate,
            bit_rate: self.bit_rate,
            speed_ratio: self.speed_ratio,
            volume_ratio: self.volume_ratio,
            pitch_ratio: self.pitch_ratio,
            emotion: self.emotion.clone(),
            language: self.language.clone(),
        };

        let tts = self.client.tts_v2();
        let stream = tts
            .stream(&req)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao tts open stream: {}", e)))?;
        pin_mut!(stream);

        while let Some(item) = stream.next().await {
            let chunk = item
                .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao tts stream chunk: {}", e)))?;
            emitter.emit(chunk.audio).await?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{MessageChunk, Part, Role};
    use tokio::sync::Mutex;

    struct MockProvider {
        mime: String,
        responses: Mutex<Vec<Result<Vec<Vec<u8>>, GenxError>>>,
    }

    #[async_trait]
    impl TtsProvider for MockProvider {
        fn mime_type(&self) -> &str {
            &self.mime
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
    async fn tts_passthrough_non_text_chunk() {
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/ogg".to_string(),
            responses: Mutex::new(vec![Ok(vec![vec![9, 9]])]),
        });
        let t = DoubaoTtsSeedV2Transformer::new_with_provider(provider);

        let passthrough = MessageChunk::blob(Role::User, "audio/pcm", vec![1, 2, 3]);
        let mut out = t
            .transform(
                "doubao/seed-v2",
                input_stream(vec![
                    passthrough.clone(),
                    MessageChunk::text(Role::User, "你好"),
                    MessageChunk::new_text_end_of_stream(),
                ]),
            )
            .await
            .expect("transform");

        let c1 = out.next().await.expect("next1").expect("c1");
        assert_eq!(c1.part, passthrough.part);

        let _bos = out.next().await.expect("bos").expect("bos chunk");
        let c3 = out.next().await.expect("audio").expect("audio chunk");
        assert!(matches!(c3.part, Some(Part::Blob(_))));
        let eos = out.next().await.expect("eos").expect("eos chunk");
        assert!(eos.is_end_of_stream());
    }
}
