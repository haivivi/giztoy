//! Stream tee — duplicate a Stream to a StreamBuilder.

use async_trait::async_trait;

use crate::error::GenxError;
use crate::stream::{Stream, StreamBuilder, StreamResult};
use crate::types::MessageChunk;

/// A Stream that reads from `src` and copies all chunks to `builder`.
/// The original chunks pass through unchanged.
pub struct TeeStream {
    src: Box<dyn Stream>,
    builder: StreamBuilder,
}

/// Create a tee: reads from `src`, copies to `builder`, returns chunks to caller.
pub fn tee(src: Box<dyn Stream>, builder: StreamBuilder) -> TeeStream {
    TeeStream { src, builder }
}

#[async_trait]
impl Stream for TeeStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        match self.src.next().await {
            Ok(Some(chunk)) => {
                let _ = self.builder.add(std::slice::from_ref(&chunk));
                Ok(Some(chunk))
            }
            Ok(None) => {
                let usage = self.src.result().map(|r| r.usage).unwrap_or_default();
                let _ = self.builder.done(usage);
                Ok(None)
            }
            Err(e) => {
                let _ = self.builder.abort_with_message(e.to_string());
                Err(e)
            }
        }
    }

    fn result(&self) -> Option<StreamResult> {
        self.src.result()
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        let usage = self.src.result().map(|r| r.usage).unwrap_or_default();
        let _ = self.builder.done(usage);
        self.src.close().await
    }

    async fn close_with_error(&mut self, error: GenxError) -> Result<(), GenxError> {
        let _ = self.builder.abort_with_message(error.to_string());
        self.src.close_with_error(error).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::Usage;
    use crate::stream::{collect_text, StreamBuilder};
    use crate::types::Role;
    use crate::error::Usage;

    #[tokio::test]
    async fn t7_1_tee_three_chunks() {
        let src_builder = StreamBuilder::with_tools(64, vec![]);
        src_builder
            .add(&[
                MessageChunk::text(Role::Model, "a"),
                MessageChunk::text(Role::Model, "b"),
                MessageChunk::text(Role::Model, "c"),
            ])
            .unwrap();
        let mut expected_usage = Usage::default();
        expected_usage.generated_token_count = 3;
        src_builder.done(expected_usage).unwrap();

        let copy_builder = StreamBuilder::with_tools(64, vec![]);
        let mut copy_stream = copy_builder.stream();

        let mut tee_stream = tee(Box::new(src_builder.stream()), copy_builder);

        let main_text = collect_text(&mut tee_stream).await.unwrap();
        assert_eq!(main_text, "abc");
        assert_eq!(tee_stream.result().unwrap().usage.generated_token_count, 3);

        let copy_text = collect_text(&mut copy_stream).await.unwrap();
        assert_eq!(copy_text, "abc");
        assert_eq!(copy_stream.result().unwrap().usage.generated_token_count, 3);
    }

    #[tokio::test]
    async fn t7_2_tee_empty_stream() {
        let src_builder = StreamBuilder::with_tools(64, vec![]);
        src_builder.done(Usage::default()).unwrap();

        let copy_builder = StreamBuilder::with_tools(64, vec![]);
        let mut copy_stream = copy_builder.stream();

        let mut tee_stream = tee(Box::new(src_builder.stream()), copy_builder);

        let result = tee_stream.next().await.unwrap();
        assert!(result.is_none());

        let copy_result = copy_stream.next().await.unwrap();
        assert!(copy_result.is_none());
    }

    #[tokio::test]
    async fn t7_3_drop_one_consumer_other_unaffected() {
        let src_builder = StreamBuilder::with_tools(64, vec![]);
        src_builder
            .add(&[
                MessageChunk::text(Role::Model, "x"),
                MessageChunk::text(Role::Model, "y"),
            ])
            .unwrap();
        src_builder.done(Usage::default()).unwrap();

        let copy_builder = StreamBuilder::with_tools(64, vec![]);
        // Drop copy_stream immediately — don't consume it
        let _copy_stream = copy_builder.stream();

        let mut tee_stream = tee(Box::new(src_builder.stream()), copy_builder);

        // Main consumer should still work fine
        let text = collect_text(&mut tee_stream).await.unwrap();
        assert_eq!(text, "xy");
    }

    #[tokio::test]
    async fn t7_4_source_error_propagates() {
        let src_builder = StreamBuilder::with_tools(64, vec![]);
        src_builder
            .abort_with_message("source error")
            .unwrap();

        let copy_builder = StreamBuilder::with_tools(64, vec![]);
        let _copy_stream = copy_builder.stream();

        let mut tee_stream = tee(Box::new(src_builder.stream()), copy_builder);

        // Read should get the error from the aborted source
        let result = tee_stream.next().await;
        assert!(result.is_err(), "expected error from aborted source, got {:?}", result);
    }
}
