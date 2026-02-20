//! Transformer trait for stream-to-stream conversion.
//!
//! # Contract
//!
//! Transformers may modify any field of MessageChunk:
//!   - Role: e.g., realtime model changes user -> model
//!   - Name: e.g., set to model name
//!   - Part: e.g., TTS converts Text -> Blob, ASR converts Blob -> Text
//!   - Ctrl: preserve or modify as needed
//!
//! # Lifecycle
//!
//! The `transform` method handles initialization only. Once it returns, the
//! background task's lifetime is governed entirely by the input Stream:
//! `input.next()` returning `None` or `Err` terminates the task.
//!
//! To cancel a running transformer, drop the input Stream.
//!
//! # EOF vs EoS
//!
//! - **EOF** (`input.next()` returns `Ok(None)`): Stream is physically done.
//!   Transformer flushes, emits results, returns. No EoS marker fabricated.
//! - **EoS marker** (`ctrl.end_of_stream == true`): Logical sub-stream boundary.
//!   Transformer flushes, emits results, emits translated EoS marker, continues.

use async_trait::async_trait;

use crate::error::GenxError;
use crate::stream::Stream;

/// Transformer converts an input Stream into an output Stream.
#[async_trait]
pub trait Transformer: Send + Sync {
    /// Create an output Stream from an input Stream.
    ///
    /// The `pattern` identifies the model/voice/resource (e.g., "doubao/vv",
    /// "minimax/shaonv"). Concrete implementations may ignore the pattern.
    ///
    /// This method should complete initialization (connection, handshake)
    /// before returning. Processing errors are returned via `Stream::next()`.
    async fn transform(
        &self,
        pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError>;
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::stream::StreamResult;
    use crate::types::{MessageChunk, Role};
    use tokio::sync::mpsc;

    struct ChannelStream {
        rx: mpsc::Receiver<Result<MessageChunk, String>>,
    }

    #[async_trait]
    impl Stream for ChannelStream {
        async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
            match self.rx.recv().await {
                Some(Ok(chunk)) => Ok(Some(chunk)),
                Some(Err(e)) => Err(GenxError::Other(anyhow::anyhow!("{}", e))),
                None => Ok(None),
            }
        }
        fn result(&self) -> Option<StreamResult> {
            None
        }
        async fn close(&mut self) -> Result<(), GenxError> {
            self.rx.close();
            Ok(())
        }
        async fn close_with_error(&mut self, _: GenxError) -> Result<(), GenxError> {
            self.rx.close();
            Ok(())
        }
    }

    struct PassthroughTransformer;

    #[async_trait]
    impl Transformer for PassthroughTransformer {
        async fn transform(
            &self,
            _pattern: &str,
            input: Box<dyn Stream>,
        ) -> Result<Box<dyn Stream>, GenxError> {
            let (tx, rx) = mpsc::channel(64);

            tokio::spawn(async move {
                let mut input = input;
                loop {
                    match input.next().await {
                        Ok(Some(chunk)) => {
                            if tx.send(Ok(chunk)).await.is_err() {
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

            Ok(Box::new(ChannelStream { rx }))
        }
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
        Box::new(ChannelStream { rx })
    }

    #[tokio::test]
    async fn t3_1_passthrough() {
        let input = make_input(vec![
            MessageChunk::text(Role::Model, "hello"),
            MessageChunk::text(Role::Model, " world"),
        ]);

        let t = PassthroughTransformer;
        let mut output = t.transform("test", input).await.unwrap();

        let mut text = String::new();
        while let Ok(Some(chunk)) = output.next().await {
            if let Some(part) = &chunk.part {
                if let Some(t) = part.as_text() {
                    text.push_str(t);
                }
            }
        }
        assert_eq!(text, "hello world");
    }

    struct FailingTransformer;

    #[async_trait]
    impl Transformer for FailingTransformer {
        async fn transform(
            &self,
            _pattern: &str,
            _input: Box<dyn Stream>,
        ) -> Result<Box<dyn Stream>, GenxError> {
            Err(GenxError::Other(anyhow::anyhow!("connection refused")))
        }
    }

    #[tokio::test]
    async fn t3_2_init_error_propagates() {
        let input = make_input(vec![]);
        let t = FailingTransformer;
        let result = t.transform("test", input).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn t3_3_input_eof_output_eof() {
        let input = make_input(vec![]);
        let t = PassthroughTransformer;
        let mut output = t.transform("test", input).await.unwrap();
        let result = output.next().await;
        assert!(matches!(result, Ok(None)));
    }
}
