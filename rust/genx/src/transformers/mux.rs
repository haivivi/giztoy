//! Transformer multiplexer and helper types.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;

use crate::error::GenxError;
use crate::stream::Stream;
use crate::transformer::Transformer;

/// Transformer multiplexer that routes transform requests to registered
/// transformers based on pattern matching.
pub struct TransformerMux {
    routes: HashMap<String, Arc<dyn Transformer>>,
    tts_routes: HashMap<String, Arc<dyn Transformer>>,
    asr_routes: HashMap<String, Arc<dyn Transformer>>,
}

impl TransformerMux {
    pub fn new() -> Self {
        Self {
            routes: HashMap::new(),
            tts_routes: HashMap::new(),
            asr_routes: HashMap::new(),
        }
    }

    /// Register a transformer for the given pattern.
    pub fn handle(
        &mut self,
        pattern: impl Into<String>,
        t: Arc<dyn Transformer>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        if self.routes.contains_key(&pattern) {
            return Err(GenxError::Other(anyhow::anyhow!(
                "transformers: transformer already registered for {}",
                pattern,
            )));
        }
        self.routes.insert(pattern, t);
        Ok(())
    }

    /// Register a TTS transformer.
    pub fn handle_tts(
        &mut self,
        pattern: impl Into<String>,
        t: Arc<dyn Transformer>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        self.tts_routes.insert(pattern.clone(), t.clone());
        if !self.routes.contains_key(&pattern) {
            self.routes.insert(pattern, t);
        }
        Ok(())
    }

    /// Register an ASR transformer.
    pub fn handle_asr(
        &mut self,
        pattern: impl Into<String>,
        t: Arc<dyn Transformer>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        self.asr_routes.insert(pattern.clone(), t.clone());
        if !self.routes.contains_key(&pattern) {
            self.routes.insert(pattern, t);
        }
        Ok(())
    }

    fn get(&self, pattern: &str) -> Result<&dyn Transformer, GenxError> {
        self.routes
            .get(pattern)
            .map(|t| t.as_ref())
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!(
                    "transformers: transformer not found for {}",
                    pattern,
                ))
            })
    }

    /// Get a TTS transformer by pattern.
    pub fn get_tts(&self, pattern: &str) -> Result<&dyn Transformer, GenxError> {
        self.tts_routes
            .get(pattern)
            .map(|t| t.as_ref())
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!(
                    "transformers: TTS transformer not found for {}",
                    pattern,
                ))
            })
    }

    /// Get an ASR transformer by pattern.
    pub fn get_asr(&self, pattern: &str) -> Result<&dyn Transformer, GenxError> {
        self.asr_routes
            .get(pattern)
            .map(|t| t.as_ref())
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!(
                    "transformers: ASR transformer not found for {}",
                    pattern,
                ))
            })
    }
}

impl Default for TransformerMux {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl Transformer for TransformerMux {
    async fn transform(
        &self,
        pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let t = self.get(pattern)?;
        t.transform(pattern, input).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::stream::StreamResult;
    use crate::types::{MessageChunk, Role};
    use tokio::sync::mpsc;

    struct EchoTransformer {
        tag: String,
    }

    #[async_trait]
    impl Transformer for EchoTransformer {
        async fn transform(
            &self,
            _pattern: &str,
            input: Box<dyn Stream>,
        ) -> Result<Box<dyn Stream>, GenxError> {
            let tag = self.tag.clone();
            let (tx, rx) = mpsc::channel(64);

            tokio::spawn(async move {
                let mut input = input;
                loop {
                    match input.next().await {
                        Ok(Some(chunk)) => {
                            let tagged = MessageChunk::text(
                                chunk.role,
                                format!(
                                    "[{}]{}",
                                    tag,
                                    chunk.part.as_ref().and_then(|p| p.as_text()).unwrap_or("")
                                ),
                            );
                            if tx.send(Ok(tagged)).await.is_err() {
                                break;
                            }
                        }
                        Ok(None) => break,
                        Err(e) => {
                            let _ = tx.send(Err(e.to_string())).await;
                            break;
                        }
                    }
                }
            });

            Ok(Box::new(TestChannelStream { rx }))
        }
    }

    struct TestChannelStream {
        rx: mpsc::Receiver<Result<MessageChunk, String>>,
    }

    #[async_trait]
    impl Stream for TestChannelStream {
        async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
            match self.rx.recv().await {
                Some(Ok(c)) => Ok(Some(c)),
                Some(Err(e)) => Err(GenxError::Other(anyhow::anyhow!("{}", e))),
                None => Ok(None),
            }
        }
        fn result(&self) -> Option<StreamResult> { None }
        async fn close(&mut self) -> Result<(), GenxError> {
            self.rx.close();
            Ok(())
        }
        async fn close_with_error(&mut self, _: GenxError) -> Result<(), GenxError> {
            self.rx.close();
            Ok(())
        }
    }

    fn make_input(chunks: Vec<MessageChunk>) -> Box<dyn Stream> {
        let (tx, rx) = mpsc::channel(64);
        tokio::spawn(async move {
            for c in chunks {
                if tx.send(Ok(c)).await.is_err() { break; }
            }
        });
        Box::new(TestChannelStream { rx })
    }

    #[tokio::test]
    async fn t14_1_register_and_transform() {
        let mut mux = TransformerMux::new();
        mux.handle("tts/test", Arc::new(EchoTransformer { tag: "TTS".into() }))
            .unwrap();

        let input = make_input(vec![MessageChunk::text(Role::Model, "hello")]);
        let mut output = mux.transform("tts/test", input).await.unwrap();

        let chunk = output.next().await.unwrap().unwrap();
        assert_eq!(chunk.part.unwrap().as_text().unwrap(), "[TTS]hello");
    }

    #[tokio::test]
    async fn t14_2_not_found() {
        let mux = TransformerMux::new();
        let input = make_input(vec![]);
        let result = mux.transform("missing", input).await;
        assert!(result.is_err());
        let err = result.err().unwrap();
        assert!(err.to_string().contains("not found"));
    }

    #[tokio::test]
    async fn t14_3_tts_mux() {
        let mut mux = TransformerMux::new();
        mux.handle_tts("tts/a", Arc::new(EchoTransformer { tag: "A".into() }))
            .unwrap();
        assert!(mux.get_tts("tts/a").is_ok());
        assert!(mux.get_tts("tts/b").is_err());
        // Also registered in main routes
        assert!(mux.get("tts/a").is_ok());
    }

    #[tokio::test]
    async fn t14_4_asr_mux() {
        let mut mux = TransformerMux::new();
        mux.handle_asr("asr/zh", Arc::new(EchoTransformer { tag: "ASR".into() }))
            .unwrap();
        assert!(mux.get_asr("asr/zh").is_ok());
        assert!(mux.get_asr("asr/en").is_err());
    }
}
