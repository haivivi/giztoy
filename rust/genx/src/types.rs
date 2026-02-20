//! Core types for the GenX LLM interface.
//!
//! This module defines the fundamental types used for representing messages,
//! roles, and content in LLM conversations.

use serde::{Deserialize, Serialize};

/// Role of a message in a conversation.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Role {
    /// User message
    User,
    /// Model/assistant message
    Model,
    /// Tool response message
    Tool,
}

impl std::fmt::Display for Role {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Role::User => write!(f, "user"),
            Role::Model => write!(f, "model"),
            Role::Tool => write!(f, "tool"),
        }
    }
}

/// A part of message content.
#[derive(Debug, Clone, PartialEq)]
pub enum Part {
    /// Text content
    Text(String),
    /// Binary blob with MIME type
    Blob(Blob),
}

impl Part {
    /// Create a new text part.
    pub fn text(s: impl Into<String>) -> Self {
        Part::Text(s.into())
    }

    /// Create a new blob part.
    pub fn blob(mime_type: impl Into<String>, data: impl Into<Vec<u8>>) -> Self {
        Part::Blob(Blob {
            mime_type: mime_type.into(),
            data: data.into(),
        })
    }

    /// Returns true if this is a text part.
    pub fn is_text(&self) -> bool {
        matches!(self, Part::Text(_))
    }

    /// Returns true if this is a blob part.
    pub fn is_blob(&self) -> bool {
        matches!(self, Part::Blob(_))
    }

    /// Get the text content if this is a text part.
    pub fn as_text(&self) -> Option<&str> {
        match self {
            Part::Text(s) => Some(s),
            _ => None,
        }
    }

    /// Get the blob if this is a blob part.
    pub fn as_blob(&self) -> Option<&Blob> {
        match self {
            Part::Blob(b) => Some(b),
            _ => None,
        }
    }
}

/// Binary blob with MIME type.
#[derive(Debug, Clone, PartialEq)]
pub struct Blob {
    /// MIME type of the blob (e.g., "image/png", "audio/mp3")
    pub mime_type: String,
    /// Binary data
    pub data: Vec<u8>,
}

/// Contents of a message, consisting of multiple parts.
pub type Contents = Vec<Part>;

/// Function call information.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct FuncCall {
    /// Name of the function to call
    pub name: String,
    /// JSON-encoded arguments
    pub arguments: String,
}

impl FuncCall {
    /// Create a new function call.
    pub fn new(name: impl Into<String>, arguments: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            arguments: arguments.into(),
        }
    }

    /// Parse the arguments as a specific type.
    pub fn parse_args<T: serde::de::DeserializeOwned>(&self) -> Result<T, serde_json::Error> {
        serde_json::from_str(&self.arguments)
    }
}

/// A tool call made by the model.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ToolCall {
    /// Unique identifier for this tool call
    pub id: String,
    /// Index of this tool call in a batch (used for streaming)
    #[serde(default)]
    pub index: i64,
    /// The function call details
    pub func_call: FuncCall,
}

impl ToolCall {
    /// Create a new tool call.
    pub fn new(id: impl Into<String>, func_call: FuncCall) -> Self {
        Self {
            id: id.into(),
            index: 0,
            func_call,
        }
    }

    /// Create a new tool call with an index.
    pub fn with_index(id: impl Into<String>, index: i64, func_call: FuncCall) -> Self {
        Self {
            id: id.into(),
            index,
            func_call,
        }
    }
}

/// Result of a tool invocation.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ToolResult {
    /// The ID of the tool call this result corresponds to
    pub id: String,
    /// The result as a string (typically JSON)
    pub result: String,
}

impl ToolResult {
    /// Create a new tool result.
    pub fn new(id: impl Into<String>, result: impl Into<String>) -> Self {
        Self {
            id: id.into(),
            result: result.into(),
        }
    }

    /// Create a tool result from a serializable value.
    pub fn from_value<T: Serialize>(id: impl Into<String>, value: &T) -> Result<Self, serde_json::Error> {
        Ok(Self {
            id: id.into(),
            result: serde_json::to_string(value)?,
        })
    }
}

/// Payload of a message - can be content, tool call, or tool result.
#[derive(Debug, Clone, PartialEq)]
pub enum Payload {
    /// Regular content (text and/or blobs)
    Contents(Contents),
    /// A tool call from the model
    ToolCall(ToolCall),
    /// A result from tool execution
    ToolResult(ToolResult),
}

impl Payload {
    /// Create a payload with text content.
    pub fn text(s: impl Into<String>) -> Self {
        Payload::Contents(vec![Part::Text(s.into())])
    }

    /// Create a payload with blob content.
    pub fn blob(mime_type: impl Into<String>, data: impl Into<Vec<u8>>) -> Self {
        Payload::Contents(vec![Part::blob(mime_type, data)])
    }

    /// Returns true if this is a contents payload.
    pub fn is_contents(&self) -> bool {
        matches!(self, Payload::Contents(_))
    }

    /// Returns true if this is a tool call payload.
    pub fn is_tool_call(&self) -> bool {
        matches!(self, Payload::ToolCall(_))
    }

    /// Returns true if this is a tool result payload.
    pub fn is_tool_result(&self) -> bool {
        matches!(self, Payload::ToolResult(_))
    }

    /// Get the contents if this is a contents payload.
    pub fn as_contents(&self) -> Option<&Contents> {
        match self {
            Payload::Contents(c) => Some(c),
            _ => None,
        }
    }

    /// Get the tool call if this is a tool call payload.
    pub fn as_tool_call(&self) -> Option<&ToolCall> {
        match self {
            Payload::ToolCall(tc) => Some(tc),
            _ => None,
        }
    }

    /// Get the tool result if this is a tool result payload.
    pub fn as_tool_result(&self) -> Option<&ToolResult> {
        match self {
            Payload::ToolResult(tr) => Some(tr),
            _ => None,
        }
    }
}

/// A complete message in a conversation.
#[derive(Debug, Clone, PartialEq)]
pub struct Message {
    /// Role of the message sender
    pub role: Role,
    /// Optional name of the sender
    pub name: Option<String>,
    /// Message payload
    pub payload: Payload,
}

impl Message {
    /// Create a new message.
    pub fn new(role: Role, payload: Payload) -> Self {
        Self {
            role,
            name: None,
            payload,
        }
    }

    /// Create a new message with a name.
    pub fn with_name(role: Role, name: impl Into<String>, payload: Payload) -> Self {
        Self {
            role,
            name: Some(name.into()),
            payload,
        }
    }

    /// Create a user text message.
    pub fn user_text(text: impl Into<String>) -> Self {
        Self::new(Role::User, Payload::text(text))
    }

    /// Create a model text message.
    pub fn model_text(text: impl Into<String>) -> Self {
        Self::new(Role::Model, Payload::text(text))
    }

    /// Create a tool call message.
    pub fn tool_call(tool_call: ToolCall) -> Self {
        Self::new(Role::Model, Payload::ToolCall(tool_call))
    }

    /// Create a tool result message.
    pub fn tool_result(tool_result: ToolResult) -> Self {
        Self::new(Role::Tool, Payload::ToolResult(tool_result))
    }
}

/// Stream control signals for routing and state.
///
/// Used by input/output modules to:
///   - Route chunks to different streams via `stream_id`
///   - Signal begin/end of a logical stream via `begin_of_stream`/`end_of_stream`
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct StreamCtrl {
    /// Sub-stream identifier for mux/demux routing.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub stream_id: String,

    /// Human-readable tag for debugging. Not used for routing or logic.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub label: String,

    /// Marks the start of a logical stream.
    #[serde(default, skip_serializing_if = "is_false")]
    pub begin_of_stream: bool,

    /// Marks the end of a logical stream.
    #[serde(default, skip_serializing_if = "is_false")]
    pub end_of_stream: bool,

    /// Unix epoch time in milliseconds when this chunk was created.
    #[serde(default, skip_serializing_if = "is_zero_i64")]
    pub timestamp: i64,
}

fn is_false(v: &bool) -> bool {
    !v
}

fn is_zero_i64(v: &i64) -> bool {
    *v == 0
}

/// A chunk of a streaming message.
///
/// When a `MessageChunk` passes through a Transformer, the Transformer MUST:
///   - Preserve `role`, `name`, `tool_call`, and `ctrl` fields unchanged
///   - Only modify the `part` field (content payload)
#[derive(Debug, Clone, PartialEq)]
pub struct MessageChunk {
    /// Role of the message sender
    pub role: Role,
    /// Optional name of the sender
    pub name: Option<String>,
    /// Content part (if any)
    pub part: Option<Part>,
    /// Tool call (if any)
    pub tool_call: Option<ToolCall>,
    /// Stream control signals (optional, for routing and state)
    pub ctrl: Option<StreamCtrl>,
}

impl MessageChunk {
    /// Create a new text chunk.
    pub fn text(role: Role, text: impl Into<String>) -> Self {
        Self {
            role,
            name: None,
            part: Some(Part::Text(text.into())),
            tool_call: None,
            ctrl: None,
        }
    }

    /// Create a new blob chunk.
    pub fn blob(role: Role, mime_type: impl Into<String>, data: impl Into<Vec<u8>>) -> Self {
        Self {
            role,
            name: None,
            part: Some(Part::blob(mime_type, data)),
            tool_call: None,
            ctrl: None,
        }
    }

    /// Create a new tool call chunk.
    pub fn tool_call(role: Role, tool_call: ToolCall) -> Self {
        Self {
            role,
            name: None,
            part: None,
            tool_call: Some(tool_call),
            ctrl: None,
        }
    }

    /// Returns true if this chunk is a begin-of-stream marker.
    pub fn is_begin_of_stream(&self) -> bool {
        self.ctrl.as_ref().is_some_and(|c| c.begin_of_stream)
    }

    /// Returns true if this chunk is an end-of-stream marker.
    pub fn is_end_of_stream(&self) -> bool {
        self.ctrl.as_ref().is_some_and(|c| c.end_of_stream)
    }

    /// Create a BOS marker with the given stream ID.
    pub fn new_begin_of_stream(stream_id: impl Into<String>) -> Self {
        Self {
            role: Role::User,
            name: None,
            part: None,
            tool_call: None,
            ctrl: Some(StreamCtrl {
                stream_id: stream_id.into(),
                begin_of_stream: true,
                ..Default::default()
            }),
        }
    }

    /// Create an EOS marker with the given MIME type.
    pub fn new_end_of_stream(mime_type: impl Into<String>) -> Self {
        Self {
            role: Role::User,
            name: None,
            part: Some(Part::Blob(Blob {
                mime_type: mime_type.into(),
                data: Vec::new(),
            })),
            tool_call: None,
            ctrl: Some(StreamCtrl {
                end_of_stream: true,
                ..Default::default()
            }),
        }
    }

    /// Create a text EOS marker.
    pub fn new_text_end_of_stream() -> Self {
        Self {
            role: Role::User,
            name: None,
            part: Some(Part::Text(String::new())),
            tool_call: None,
            ctrl: Some(StreamCtrl {
                end_of_stream: true,
                ..Default::default()
            }),
        }
    }

    /// Clone this chunk.
    pub fn clone_chunk(&self) -> Self {
        self.clone()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_role_display() {
        assert_eq!(Role::User.to_string(), "user");
        assert_eq!(Role::Model.to_string(), "model");
        assert_eq!(Role::Tool.to_string(), "tool");
    }

    #[test]
    fn test_part_constructors() {
        let text = Part::text("hello");
        assert!(text.is_text());
        assert_eq!(text.as_text(), Some("hello"));

        let blob = Part::blob("image/png", vec![1, 2, 3]);
        assert!(blob.is_blob());
        assert!(blob.as_blob().is_some());
    }

    #[test]
    fn test_message_constructors() {
        let user_msg = Message::user_text("Hello!");
        assert_eq!(user_msg.role, Role::User);
        assert!(user_msg.payload.is_contents());

        let model_msg = Message::model_text("Hi there!");
        assert_eq!(model_msg.role, Role::Model);
    }

    #[test]
    fn test_func_call_parse_args() {
        #[derive(Debug, PartialEq, Deserialize)]
        struct Args {
            name: String,
            count: i32,
        }

        let fc = FuncCall::new("test", r#"{"name": "foo", "count": 42}"#);
        let args: Args = fc.parse_args().unwrap();
        assert_eq!(args.name, "foo");
        assert_eq!(args.count, 42);
    }

    #[test]
    fn test_tool_result_from_value() {
        #[derive(Serialize)]
        struct Result {
            success: bool,
        }

        let result = ToolResult::from_value("call_123", &Result { success: true }).unwrap();
        assert_eq!(result.id, "call_123");
        assert!(result.result.contains("true"));
    }

    // T1: StreamCtrl tests

    #[test]
    fn t1_1_stream_ctrl_default_zero_values() {
        let ctrl = StreamCtrl::default();
        assert_eq!(ctrl.stream_id, "");
        assert_eq!(ctrl.label, "");
        assert!(!ctrl.begin_of_stream);
        assert!(!ctrl.end_of_stream);
        assert_eq!(ctrl.timestamp, 0);
    }

    #[test]
    fn t1_2_no_ctrl_not_bos_eos() {
        let chunk = MessageChunk::text(Role::Model, "hello");
        assert!(!chunk.is_begin_of_stream());
        assert!(!chunk.is_end_of_stream());
    }

    #[test]
    fn t1_3_bos_ctrl() {
        let mut chunk = MessageChunk::text(Role::Model, "hello");
        chunk.ctrl = Some(StreamCtrl {
            begin_of_stream: true,
            ..Default::default()
        });
        assert!(chunk.is_begin_of_stream());
        assert!(!chunk.is_end_of_stream());
    }

    #[test]
    fn t1_4_eos_ctrl() {
        let mut chunk = MessageChunk::text(Role::Model, "hello");
        chunk.ctrl = Some(StreamCtrl {
            end_of_stream: true,
            ..Default::default()
        });
        assert!(!chunk.is_begin_of_stream());
        assert!(chunk.is_end_of_stream());
    }

    #[test]
    fn t1_5_new_begin_of_stream() {
        let chunk = MessageChunk::new_begin_of_stream("s1");
        let ctrl = chunk.ctrl.as_ref().unwrap();
        assert_eq!(ctrl.stream_id, "s1");
        assert!(ctrl.begin_of_stream);
        assert!(!ctrl.end_of_stream);
    }

    #[test]
    fn t1_6_new_end_of_stream_blob() {
        let chunk = MessageChunk::new_end_of_stream("audio/pcm");
        assert!(chunk.is_end_of_stream());
        let part = chunk.part.unwrap();
        let blob = part.as_blob().unwrap();
        assert_eq!(blob.mime_type, "audio/pcm");
        assert!(blob.data.is_empty());
    }

    #[test]
    fn t1_7_new_text_end_of_stream() {
        let chunk = MessageChunk::new_text_end_of_stream();
        assert!(chunk.is_end_of_stream());
        let part = chunk.part.unwrap();
        assert_eq!(part.as_text(), Some(""));
    }

    #[test]
    fn t1_8_stream_ctrl_clone_eq() {
        let ctrl = StreamCtrl {
            stream_id: "abc".into(),
            label: "test".into(),
            begin_of_stream: true,
            end_of_stream: false,
            timestamp: 12345,
        };
        let cloned = ctrl.clone();
        assert_eq!(ctrl, cloned);
    }

    #[test]
    fn t1_9_message_chunk_clone_with_ctrl() {
        let mut chunk = MessageChunk::text(Role::Model, "hello");
        chunk.ctrl = Some(StreamCtrl {
            stream_id: "s1".into(),
            begin_of_stream: true,
            ..Default::default()
        });
        let cloned = chunk.clone();
        assert_eq!(chunk, cloned);
        assert!(cloned.ctrl.is_some());
        assert_eq!(cloned.ctrl.unwrap().stream_id, "s1");
    }

    #[test]
    fn t1_stream_ctrl_json_roundtrip() {
        let ctrl = StreamCtrl {
            stream_id: "abc123".into(),
            label: "debug".into(),
            begin_of_stream: true,
            end_of_stream: false,
            timestamp: 1700000000000,
        };
        let json = serde_json::to_string(&ctrl).unwrap();
        let parsed: StreamCtrl = serde_json::from_str(&json).unwrap();
        assert_eq!(ctrl, parsed);
    }

    #[test]
    fn t1_stream_ctrl_json_skip_defaults() {
        let ctrl = StreamCtrl {
            stream_id: "abc".into(),
            ..Default::default()
        };
        let json = serde_json::to_string(&ctrl).unwrap();
        assert!(!json.contains("label"));
        assert!(!json.contains("begin_of_stream"));
        assert!(!json.contains("end_of_stream"));
        assert!(!json.contains("timestamp"));
    }
}
