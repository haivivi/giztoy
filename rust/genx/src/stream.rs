//! Streaming response handling for GenX.
//!
//! This module provides types for building and consuming streaming responses
//! from LLM generators.

use std::collections::HashMap;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};

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

/// Stream completion result containing final status and usage statistics.
#[derive(Debug, Clone)]
pub struct StreamResult {
    /// Final status of the stream
    pub status: Status,
    /// Token usage statistics
    pub usage: Usage,
    /// Refusal reason (if blocked)
    pub refusal: Option<String>,
    /// Error message (if error occurred)
    pub error: Option<String>,
}

impl StreamResult {
    /// Create a successful completion result.
    pub fn done(usage: Usage) -> Self {
        Self {
            status: Status::Done,
            usage,
            refusal: None,
            error: None,
        }
    }
}

impl Default for StreamResult {
    fn default() -> Self {
        Self {
            status: Status::Ok,
            usage: Usage::default(),
            refusal: None,
            error: None,
        }
    }
}

/// Trait for consuming streaming responses.
///
/// # Idiomatic Usage
///
/// The stream follows Rust iterator conventions:
/// - `Ok(Some(chunk))` - A message chunk is available
/// - `Ok(None)` - Stream is exhausted (no more data)
/// - `Err(e)` - An error occurred
///
/// After the stream is exhausted (`Ok(None)`), call `result()` to get
/// the final status and usage statistics.
///
/// # Example
///
/// ```ignore
/// let mut stream = generator.generate_stream("", &ctx).await?;
///
/// while let Some(chunk) = stream.next().await? {
///     if let Some(text) = chunk.part.and_then(|p| p.as_text()) {
///         print!("{}", text);
///     }
/// }
///
/// // Get final usage after stream completes
/// if let Some(result) = stream.result() {
///     println!("Tokens used: {}", result.usage.prompt_token_count);
/// }
/// ```
#[async_trait]
pub trait Stream: Send + Sync {
    /// Get the next message chunk.
    ///
    /// Returns `Ok(Some(chunk))` for each chunk, `Ok(None)` when the stream
    /// is exhausted, or `Err` on error.
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError>;

    /// Get the stream completion result after the stream is exhausted.
    ///
    /// Returns `Some(result)` after `next()` returns `Ok(None)`, containing
    /// the final status and usage statistics. Returns `None` if the stream
    /// is still active or was not properly terminated.
    fn result(&self) -> Option<StreamResult>;

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
    result: Arc<Mutex<Option<StreamResult>>>,
    done: Arc<AtomicBool>,
}

impl StreamBuilder {
    /// Create a new stream builder.
    pub fn new(_mctx: &dyn ModelContext, size: usize) -> Self {
        // Note: We can't easily downcast dyn Tool to FuncTool without additional infrastructure.
        // For now, create an empty tools map. Users can use with_tools() instead.
        Self {
            buffer: BlockBuffer::new(size),
            func_tools: HashMap::new(),
            result: Arc::new(Mutex::new(None)),
            done: Arc::new(AtomicBool::new(false)),
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
            result: Arc::new(Mutex::new(None)),
            done: Arc::new(AtomicBool::new(false)),
        }
    }

    /// Signal that the stream is done.
    pub fn done(&self, usage: Usage) -> Result<(), giztoy_buffer::BufferError> {
        if self.done.swap(true, Ordering::SeqCst) {
            return Ok(()); // Already done
        }
        *self.result.lock().unwrap() = Some(StreamResult {
            status: Status::Done,
            usage: usage.clone(),
            refusal: None,
            error: None,
        });
        self.buffer.write(&[StreamEvent::done(usage)])?;
        self.buffer.close_write()
    }

    /// Signal that the response was truncated.
    pub fn truncated(&self, usage: Usage) -> Result<(), giztoy_buffer::BufferError> {
        if self.done.swap(true, Ordering::SeqCst) {
            return Ok(());
        }
        *self.result.lock().unwrap() = Some(StreamResult {
            status: Status::Truncated,
            usage: usage.clone(),
            refusal: None,
            error: None,
        });
        self.buffer.write(&[StreamEvent::truncated(usage)])?;
        self.buffer.close_write()
    }

    /// Signal that the response was blocked.
    pub fn blocked(
        &self,
        usage: Usage,
        refusal: impl Into<String>,
    ) -> Result<(), giztoy_buffer::BufferError> {
        if self.done.swap(true, Ordering::SeqCst) {
            return Ok(());
        }
        let refusal = refusal.into();
        *self.result.lock().unwrap() = Some(StreamResult {
            status: Status::Blocked,
            usage: usage.clone(),
            refusal: Some(refusal.clone()),
            error: None,
        });
        self.buffer.write(&[StreamEvent::blocked(usage, refusal)])?;
        self.buffer.close_write()
    }

    /// Signal an unexpected error.
    pub fn unexpected(
        &self,
        usage: Usage,
        error: impl Into<String>,
    ) -> Result<(), giztoy_buffer::BufferError> {
        if self.done.swap(true, Ordering::SeqCst) {
            return Ok(());
        }
        let error = error.into();
        *self.result.lock().unwrap() = Some(StreamResult {
            status: Status::Error,
            usage: usage.clone(),
            refusal: None,
            error: Some(error.clone()),
        });
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
    pub fn abort(
        &self,
        error: impl std::error::Error + Send + Sync + 'static,
    ) -> Result<(), giztoy_buffer::BufferError> {
        if self.done.swap(true, Ordering::SeqCst) {
            return Ok(());
        }
        *self.result.lock().unwrap() = Some(StreamResult {
            status: Status::Error,
            usage: Usage::default(),
            refusal: None,
            error: Some(error.to_string()),
        });
        self.buffer.close_with_error(error)
    }

    /// Abort the stream with an error message.
    pub fn abort_with_message(
        &self,
        message: impl Into<String>,
    ) -> Result<(), giztoy_buffer::BufferError> {
        if self.done.swap(true, Ordering::SeqCst) {
            return Ok(());
        }
        let msg = message.into();
        *self.result.lock().unwrap() = Some(StreamResult {
            status: Status::Error,
            usage: Usage::default(),
            refusal: None,
            error: Some(msg.clone()),
        });
        self.buffer
            .close_with_error(GenxError::Other(anyhow::anyhow!("{}", msg)))
    }

    /// Get a stream consumer.
    pub fn stream(&self) -> StreamImpl {
        StreamImpl {
            buffer: self.buffer.clone(),
            result: self.result.clone(),
            local_result: None,
        }
    }
}

/// Implementation of Stream trait using BlockBuffer.
pub struct StreamImpl {
    buffer: BlockBuffer<StreamEvent>,
    result: Arc<Mutex<Option<StreamResult>>>,
    local_result: Option<StreamResult>,
}

#[async_trait]
impl Stream for StreamImpl {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        // If we already completed, return None
        if self.local_result.is_some() {
            return Ok(None);
        }

        let mut buf = [StreamEvent::done(Usage::default())];

        match self.buffer.read(&mut buf) {
            Ok(0) => {
                // Buffer exhausted without explicit done - copy result from shared state
                if let Some(result) = self.result.lock().unwrap().clone() {
                    self.local_result = Some(result);
                }
                Ok(None)
            }
            Ok(_) => {
                let evt = &buf[0];
                match evt.status {
                    Status::Ok => Ok(evt.chunk.clone()),
                    Status::Done => {
                        self.local_result = Some(StreamResult::done(evt.usage.clone()));
                        Ok(None)
                    }
                    Status::Truncated => {
                        self.local_result = Some(StreamResult {
                            status: Status::Truncated,
                            usage: evt.usage.clone(),
                            refusal: None,
                            error: None,
                        });
                        Err(GenxError::Truncated(evt.usage.clone()))
                    }
                    Status::Blocked => {
                        self.local_result = Some(StreamResult {
                            status: Status::Blocked,
                            usage: evt.usage.clone(),
                            refusal: evt.refusal.clone(),
                            error: None,
                        });
                        Err(GenxError::Blocked {
                            usage: evt.usage.clone(),
                            reason: evt.refusal.clone().unwrap_or_default(),
                        })
                    }
                    Status::Error => {
                        self.local_result = Some(StreamResult {
                            status: Status::Error,
                            usage: evt.usage.clone(),
                            refusal: None,
                            error: evt.error.clone(),
                        });
                        Err(GenxError::Generation {
                            usage: evt.usage.clone(),
                            message: evt.error.clone().unwrap_or_default(),
                        })
                    }
                }
            }
            Err(giztoy_buffer::BufferError::Closed) => {
                if let Some(result) = self.result.lock().unwrap().clone() {
                    self.local_result = Some(result);
                }
                Ok(None)
            }
            Err(giztoy_buffer::BufferError::ClosedWithError(e)) => {
                self.local_result = Some(StreamResult {
                    status: Status::Error,
                    usage: Usage::default(),
                    refusal: None,
                    error: Some(e.to_string()),
                });
                Err(GenxError::Other(anyhow::anyhow!("stream closed: {}", e)))
            }
        }
    }

    fn result(&self) -> Option<StreamResult> {
        self.local_result.clone()
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
            result: self.result.clone(),
            local_result: self.local_result.clone(),
        }
    }
}

/// Collect all text from a stream.
pub async fn collect_text(stream: &mut dyn Stream) -> Result<String, GenxError> {
    let mut text = String::new();

    while let Some(chunk) = stream.next().await? {
        if let Some(part) = &chunk.part {
            if let Some(t) = part.as_text() {
                text.push_str(t);
            }
        }
    }

    Ok(text)
}

/// Collect all tool calls from a stream.
pub async fn collect_tool_calls(stream: &mut dyn Stream) -> Result<Vec<ToolCall>, GenxError> {
    let mut tool_calls = Vec::new();

    while let Some(chunk) = stream.next().await? {
        if let Some(tc) = chunk.tool_call {
            tool_calls.push(tc);
        }
    }

    Ok(tool_calls)
}

/// Collect all tool calls from a stream, accumulating streamed tool calls.
///
/// OpenAI streams tool calls in multiple chunks with index and partial data.
/// This function accumulates them into complete ToolCall objects.
pub async fn collect_tool_calls_streamed(
    stream: &mut dyn Stream,
) -> Result<Vec<ToolCall>, GenxError> {
    use crate::types::FuncCall;

    let mut tool_calls: HashMap<i64, ToolCall> = HashMap::new();
    let mut text_content = String::new();

    while let Some(chunk) = stream.next().await? {
        // Collect text content
        if let Some(part) = &chunk.part {
            if let Some(t) = part.as_text() {
                text_content.push_str(t);
            }
        }

        // Collect tool calls with index-based accumulation
        if let Some(tc) = chunk.tool_call {
            let entry = tool_calls.entry(tc.index).or_insert_with(|| ToolCall {
                id: String::new(),
                index: tc.index,
                func_call: FuncCall {
                    name: String::new(),
                    arguments: String::new(),
                },
            });

            // Accumulate fields
            if !tc.id.is_empty() {
                entry.id = tc.id;
            }
            if !tc.func_call.name.is_empty() {
                entry.func_call.name = tc.func_call.name;
            }
            entry.func_call.arguments.push_str(&tc.func_call.arguments);
        }
    }

    // Convert to sorted Vec by index
    let mut result: Vec<ToolCall> = tool_calls.into_values().collect();
    result.sort_by_key(|tc| tc.index);
    Ok(result)
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

        while let Some(chunk) = stream.next().await.unwrap() {
            if let Some(Part::Text(t)) = chunk.part {
                text.push_str(&t);
            }
        }

        assert_eq!(text, "Hello World");

        // Check result is available after completion
        let result = stream.result().expect("result should be available");
        assert_eq!(result.status, Status::Done);
        assert_eq!(result.usage.prompt_token_count, 10);
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

    #[tokio::test]
    async fn test_stream_result_after_completion() {
        let builder = StreamBuilder::with_tools(32, vec![]);
        builder
            .add(&[MessageChunk::text(Role::Model, "Test")])
            .unwrap();
        builder.done(Usage::with_counts(5, 0, 1)).unwrap();

        let mut stream = builder.stream();

        // Result should be None before exhausting stream
        assert!(stream.result().is_none());

        // Exhaust stream
        while stream.next().await.unwrap().is_some() {}

        // Result should be available now
        let result = stream.result().expect("result should be available");
        assert_eq!(result.status, Status::Done);
        assert_eq!(result.usage.prompt_token_count, 5);
        assert_eq!(result.usage.generated_token_count, 1);
    }
}
