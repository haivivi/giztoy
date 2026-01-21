//! Streaming response handling for GenX.
//!
//! This module provides types for building and consuming streaming responses
//! from LLM generators.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use giztoy_buffer::BlockBuffer;

use crate::context::ModelContext;
use crate::error::{GenxError, Status, Usage};
use crate::tool::FuncTool;
use crate::types::{MessageChunk, ToolCall};

/// A streaming event from the generator.
#[derive(Debug, Clone)]
pub struct StreamEvent {
    /// The message chunk (if any)
    pub chunk: Option<MessageChunk>,
    /// Stream status
    pub status: Status,
    /// Token usage (populated on completion)
    pub usage: Usage,
    /// Refusal message (if blocked)
    pub refusal: Option<String>,
    /// Error (if status is Error)
    pub error: Option<String>,
}

impl StreamEvent {
    /// Create a chunk event.
    pub fn chunk(chunk: MessageChunk) -> Self {
        Self {
            chunk: Some(chunk),
            status: Status::Ok,
            usage: Usage::default(),
            refusal: None,
            error: None,
        }
    }

    /// Create a done event.
    pub fn done(usage: Usage) -> Self {
        Self {
            chunk: None,
            status: Status::Done,
            usage,
            refusal: None,
            error: None,
        }
    }

    /// Create a truncated event.
    pub fn truncated(usage: Usage) -> Self {
        Self {
            chunk: None,
            status: Status::Truncated,
            usage,
            refusal: None,
            error: None,
        }
    }

    /// Create a blocked event.
    pub fn blocked(usage: Usage, refusal: impl Into<String>) -> Self {
        Self {
            chunk: None,
            status: Status::Blocked,
            usage,
            refusal: Some(refusal.into()),
            error: None,
        }
    }

    /// Create an error event.
    pub fn error(usage: Usage, error: impl Into<String>) -> Self {
        Self {
            chunk: None,
            status: Status::Error,
            usage,
            refusal: None,
            error: Some(error.into()),
        }
    }
}

/// Trait for consuming streaming responses.
#[async_trait]
pub trait Stream: Send + Sync {
    /// Get the next message chunk.
    ///
    /// Returns `Ok(Some(chunk))` for each chunk, `Ok(None)` when done,
    /// or `Err` on error.
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError>;

    /// Close the stream.
    async fn close(&mut self) -> Result<(), GenxError>;

    /// Close the stream with an error.
    async fn close_with_error(&mut self, error: GenxError) -> Result<(), GenxError>;
}

/// Builder for constructing streaming responses.
///
/// This is used by generator implementations to produce streaming output.
pub struct StreamBuilder {
    buffer: BlockBuffer<StreamEvent>,
    func_tools: HashMap<String, Arc<FuncTool>>,
}

impl StreamBuilder {
    /// Create a new stream builder.
    pub fn new(_mctx: &dyn ModelContext, size: usize) -> Self {
        // Note: We can't easily downcast dyn Tool to FuncTool without additional infrastructure.
        // For now, create an empty tools map. Users can use with_tools() instead.
        Self {
            buffer: BlockBuffer::new(size),
            func_tools: HashMap::new(),
        }
    }

    /// Create with explicit tools.
    pub fn with_tools(size: usize, tools: impl IntoIterator<Item = FuncTool>) -> Self {
        let func_tools = tools
            .into_iter()
            .map(|t| (t.name.clone(), Arc::new(t)))
            .collect();

        Self {
            buffer: BlockBuffer::new(size),
            func_tools,
        }
    }

    /// Signal that the stream is done.
    pub fn done(&self, usage: Usage) -> Result<(), giztoy_buffer::BufferError> {
        self.buffer.write(&[StreamEvent::done(usage)])?;
        self.buffer.close_write()
    }

    /// Signal that the response was truncated.
    pub fn truncated(&self, usage: Usage) -> Result<(), giztoy_buffer::BufferError> {
        self.buffer.write(&[StreamEvent::truncated(usage)])?;
        self.buffer.close_write()
    }

    /// Signal that the response was blocked.
    pub fn blocked(
        &self,
        usage: Usage,
        refusal: impl Into<String>,
    ) -> Result<(), giztoy_buffer::BufferError> {
        self.buffer.write(&[StreamEvent::blocked(usage, refusal)])?;
        self.buffer.close_write()
    }

    /// Signal an unexpected error.
    pub fn unexpected(
        &self,
        usage: Usage,
        error: impl Into<String>,
    ) -> Result<(), giztoy_buffer::BufferError> {
        self.buffer.write(&[StreamEvent::error(usage, error)])?;
        self.buffer.close_write()
    }

    /// Add message chunks to the stream.
    pub fn add(&self, chunks: &[MessageChunk]) -> Result<(), giztoy_buffer::BufferError> {
        for chunk in chunks {
            let mut chunk = chunk.clone();

            // Link tool calls to their definitions
            if let Some(ref mut tool_call) = chunk.tool_call {
                if let Some(tool) = self.func_tools.get(&tool_call.func_call.name) {
                    // Tool is available - we could attach metadata here if needed
                    let _ = tool; // silence unused warning
                }
            }

            self.buffer.write(&[StreamEvent::chunk(chunk)])?;
        }
        Ok(())
    }

    /// Abort the stream with an error.
    pub fn abort(&self, error: impl std::error::Error + Send + Sync + 'static) -> Result<(), giztoy_buffer::BufferError> {
        self.buffer.close_with_error(error)
    }

    /// Abort the stream with an error message.
    pub fn abort_with_message(&self, message: impl Into<String>) -> Result<(), giztoy_buffer::BufferError> {
        self.buffer.close_with_error(GenxError::Other(anyhow::anyhow!("{}", message.into())))
    }

    /// Get a stream consumer.
    pub fn stream(&self) -> StreamImpl {
        StreamImpl {
            buffer: self.buffer.clone(),
        }
    }
}

/// Implementation of Stream trait using BlockBuffer.
pub struct StreamImpl {
    buffer: BlockBuffer<StreamEvent>,
}

#[async_trait]
impl Stream for StreamImpl {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        let mut buf = [StreamEvent::done(Usage::default())];

        match self.buffer.read(&mut buf) {
            Ok(0) => Ok(None),
            Ok(_) => {
                let evt = &buf[0];
                match evt.status {
                    Status::Ok => Ok(evt.chunk.clone()),
                    Status::Done => Err(GenxError::Done(evt.usage.clone())),
                    Status::Truncated => Err(GenxError::Truncated(evt.usage.clone())),
                    Status::Blocked => Err(GenxError::Blocked {
                        usage: evt.usage.clone(),
                        reason: evt.refusal.clone().unwrap_or_default(),
                    }),
                    Status::Error => Err(GenxError::Generation {
                        usage: evt.usage.clone(),
                        message: evt.error.clone().unwrap_or_default(),
                    }),
                }
            }
            Err(giztoy_buffer::BufferError::Closed) => Ok(None),
            Err(giztoy_buffer::BufferError::ClosedWithError(e)) => {
                Err(GenxError::Other(anyhow::anyhow!("stream closed: {}", e)))
            }
        }
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        let _ = self.buffer.close();
        Ok(())
    }

    async fn close_with_error(&mut self, error: GenxError) -> Result<(), GenxError> {
        let _ = self.buffer.close_with_error(error);
        Ok(())
    }
}

impl Clone for StreamImpl {
    fn clone(&self) -> Self {
        Self {
            buffer: self.buffer.clone(),
        }
    }
}

/// Collect all text from a stream.
pub async fn collect_text(stream: &mut dyn Stream) -> Result<String, GenxError> {
    let mut text = String::new();

    loop {
        match stream.next().await {
            Ok(Some(chunk)) => {
                if let Some(part) = &chunk.part {
                    if let Some(t) = part.as_text() {
                        text.push_str(t);
                    }
                }
            }
            Ok(None) => break,
            Err(GenxError::Done(_)) => break,
            Err(e) => return Err(e),
        }
    }

    Ok(text)
}

/// Collect all tool calls from a stream.
pub async fn collect_tool_calls(stream: &mut dyn Stream) -> Result<Vec<ToolCall>, GenxError> {
    let mut tool_calls = Vec::new();

    loop {
        match stream.next().await {
            Ok(Some(chunk)) => {
                if let Some(tc) = chunk.tool_call {
                    tool_calls.push(tc);
                }
            }
            Ok(None) => break,
            Err(GenxError::Done(_)) => break,
            Err(e) => return Err(e),
        }
    }

    Ok(tool_calls)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{Part, Role};

    #[test]
    fn test_stream_event_constructors() {
        let chunk = StreamEvent::chunk(MessageChunk::text(Role::Model, "Hello"));
        assert_eq!(chunk.status, Status::Ok);
        assert!(chunk.chunk.is_some());

        let done = StreamEvent::done(Usage::with_counts(10, 0, 5));
        assert_eq!(done.status, Status::Done);
        assert_eq!(done.usage.prompt_token_count, 10);

        let blocked = StreamEvent::blocked(Usage::default(), "safety");
        assert_eq!(blocked.status, Status::Blocked);
        assert_eq!(blocked.refusal, Some("safety".to_string()));
    }

    #[tokio::test]
    async fn test_stream_builder_basic() {
        let builder = StreamBuilder::with_tools(32, vec![]);

        // Add some chunks
        builder
            .add(&[
                MessageChunk::text(Role::Model, "Hello"),
                MessageChunk::text(Role::Model, " World"),
            ])
            .unwrap();

        // Signal done
        builder.done(Usage::with_counts(10, 0, 2)).unwrap();

        // Read from stream
        let mut stream = builder.stream();
        let mut text = String::new();

        loop {
            match stream.next().await {
                Ok(Some(chunk)) => {
                    if let Some(Part::Text(t)) = chunk.part {
                        text.push_str(&t);
                    }
                }
                Ok(None) => break,
                Err(GenxError::Done(usage)) => {
                    assert_eq!(usage.prompt_token_count, 10);
                    break;
                }
                Err(e) => panic!("Unexpected error: {:?}", e),
            }
        }

        assert_eq!(text, "Hello World");
    }

    #[tokio::test]
    async fn test_collect_text() {
        let builder = StreamBuilder::with_tools(32, vec![]);
        builder
            .add(&[
                MessageChunk::text(Role::Model, "Hello"),
                MessageChunk::text(Role::Model, " "),
                MessageChunk::text(Role::Model, "World"),
            ])
            .unwrap();
        builder.done(Usage::default()).unwrap();

        let mut stream = builder.stream();
        let text = collect_text(&mut stream).await.unwrap();
        assert_eq!(text, "Hello World");
    }
}
