use std::sync::Arc;

use async_trait::async_trait;
use futures::{pin_mut, StreamExt};
use giztoy_minimax as minimax;

use crate::error::GenxError;
use crate::stream::Stream;
use crate::transformer::Transformer;

use super::tts_core::{spawn_tts_transform_loop, AudioEmitter, TtsProvider};

pub struct MinimaxTtsTransformer {
    provider: Arc<dyn TtsProvider>,
}

impl MinimaxTtsTransformer {
    pub fn new(client: Arc<minimax::Client>, voice_id: impl Into<String>) -> Self {
        Self {
            provider: Arc::new(MinimaxProvider {
                client,
                model: "speech-2.6-hd".to_string(),
                voice_id: voice_id.into(),
                speed: 1.0,
                vol: 1.0,
                pitch: 0,
                emotion: None,
                format: "mp3".to_string(),
                sample_rate: Some(32000),
                bitrate: Some(128000),
            }),
        }
    }

    pub(crate) fn new_with_provider(provider: Arc<dyn TtsProvider>) -> Self {
        Self { provider }
    }
}

#[async_trait]
impl Transformer for MinimaxTtsTransformer {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        Ok(spawn_tts_transform_loop(Arc::clone(&self.provider), input))
    }
}

struct MinimaxProvider {
    client: Arc<minimax::Client>,
    model: String,
    voice_id: String,
    speed: f64,
    vol: f64,
    pitch: i32,
    emotion: Option<String>,
    format: String,
    sample_rate: Option<i32>,
    bitrate: Option<i32>,
}

#[async_trait]
impl TtsProvider for MinimaxProvider {
    fn mime_type(&self) -> &str {
        match self.format.as_str() {
            "mp3" => "audio/mpeg",
            "pcm" => "audio/pcm",
            "flac" => "audio/flac",
            "wav" => "audio/wav",
            _ => "audio/mpeg",
        }
    }

    async fn synthesize_stream(
        &self,
        text: &str,
        emitter: &mut dyn AudioEmitter,
    ) -> Result<(), GenxError> {
        let req = minimax::SpeechRequest {
            model: self.model.clone(),
            text: text.to_string(),
            voice_setting: Some(minimax::VoiceSetting {
                voice_id: self.voice_id.clone(),
                speed: Some(self.speed),
                vol: Some(self.vol),
                pitch: Some(self.pitch),
                emotion: self.emotion.clone(),
            }),
            audio_setting: Some(minimax::AudioSetting {
                sample_rate: self.sample_rate,
                bitrate: self.bitrate,
                format: Some(match self.format.as_str() {
                    "pcm" => minimax::AudioFormat::Pcm,
                    "flac" => minimax::AudioFormat::Flac,
                    "wav" => minimax::AudioFormat::Wav,
                    _ => minimax::AudioFormat::Mp3,
                }),
                channel: None,
            }),
            ..Default::default()
        };

        let speech = self.client.speech();
        let stream = speech
            .synthesize_stream(&req)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("minimax tts open stream: {}", e)))?;

        pin_mut!(stream);
        while let Some(item) = stream.next().await {
            let chunk = item
                .map_err(|e| GenxError::Other(anyhow::anyhow!("minimax tts stream chunk: {}", e)))?;
            emitter.emit(chunk.audio).await?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{MessageChunk, Part, Role, StreamCtrl};
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
    async fn tts_emits_bos_audio_eos_on_text_eos() {
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/mpeg".to_string(),
            responses: Mutex::new(vec![Ok(vec![vec![1, 2], vec![3]])]),
        });
        let t = MinimaxTtsTransformer::new_with_provider(provider);

        let mut in_chunk = MessageChunk::text(Role::Model, "hello");
        in_chunk.name = Some("assistant".to_string());
        in_chunk.ctrl = Some(StreamCtrl {
            stream_id: "s1".to_string(),
            ..Default::default()
        });
        let eos = MessageChunk::new_text_end_of_stream();

        let mut out = t
            .transform("minimax/shaonv", input_stream(vec![in_chunk, eos]))
            .await
            .expect("transform");

        let c1 = out.next().await.expect("next1").expect("chunk1");
        assert!(c1.is_begin_of_stream());
        assert_eq!(c1.ctrl.as_ref().map(|c| c.stream_id.as_str()), Some("s1"));
        assert_eq!(c1.role, Role::Model);

        let c2 = out.next().await.expect("next2").expect("chunk2");
        assert!(matches!(c2.part, Some(Part::Blob(_))));
        assert_eq!(c2.ctrl.as_ref().map(|c| c.stream_id.as_str()), Some("s1"));

        let c3 = out.next().await.expect("next3").expect("chunk3");
        assert!(matches!(c3.part, Some(Part::Blob(_))));

        let c4 = out.next().await.expect("next4").expect("chunk4");
        assert!(c4.is_end_of_stream());
        assert_eq!(c4.ctrl.as_ref().map(|c| c.stream_id.as_str()), Some("s1"));
        assert!(out.next().await.expect("eof").is_none());
    }

    #[tokio::test]
    async fn tts_propagates_provider_error() {
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/mpeg".to_string(),
            responses: Mutex::new(vec![Err(GenxError::Other(anyhow::anyhow!(
                "mock synth fail"
            )))]),
        });
        let t = MinimaxTtsTransformer::new_with_provider(provider);

        let mut out = t
            .transform(
                "minimax/shaonv",
                input_stream(vec![
                    MessageChunk::text(Role::Model, "hello"),
                    MessageChunk::new_text_end_of_stream(),
                ]),
            )
            .await
            .expect("transform");

        let _ = out.next().await.expect("bos").expect("bos chunk");
        let err = out.next().await.expect_err("expect provider error");
        assert!(err.to_string().contains("mock synth fail"));
    }
}
