use std::collections::HashMap;
use std::sync::atomic::{AtomicI64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::error::MemoryError;

// ---------------------------------------------------------------------------
// Message: conversation message
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub enum Role {
    #[default]
    #[serde(rename = "user")]
    User,
    #[serde(rename = "model")]
    Model,
    #[serde(rename = "tool")]
    Tool,
}

impl std::fmt::Display for Role {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Role::User => f.write_str("user"),
            Role::Model => f.write_str("model"),
            Role::Tool => f.write_str("tool"),
        }
    }
}

/// A single conversation turn stored in short-term memory.
/// Msgpack field tags match Go for cross-language compatibility.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Message {
    #[serde(rename = "role")]
    pub role: Role,

    #[serde(rename = "name", default, skip_serializing_if = "String::is_empty")]
    pub name: String,

    #[serde(rename = "content", default, skip_serializing_if = "String::is_empty")]
    pub content: String,

    #[serde(rename = "ts")]
    pub timestamp: i64,

    #[serde(rename = "tc_id", default, skip_serializing_if = "String::is_empty")]
    pub tool_call_id: String,

    #[serde(rename = "tc_name", default, skip_serializing_if = "String::is_empty")]
    pub tool_call_name: String,

    #[serde(rename = "tc_args", default, skip_serializing_if = "String::is_empty")]
    pub tool_call_args: String,

    #[serde(rename = "tr_id", default, skip_serializing_if = "String::is_empty")]
    pub tool_result_id: String,
}

// ---------------------------------------------------------------------------
// Recall types
// ---------------------------------------------------------------------------

/// Parameters for [Memory::recall].
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RecallQuery {
    pub labels: Vec<String>,
    pub text: String,
    pub hops: usize,
    pub limit: usize,
}

/// Combined recall output.
pub struct RecallResult {
    pub entities: Vec<EntityInfo>,
    pub segments: Vec<ScoredSegment>,
}

/// Graph entity label + attributes for context building.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EntityInfo {
    pub label: String,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub attrs: HashMap<String, Value>,
}

/// Segment summary paired with relevance score.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ScoredSegment {
    pub id: String,
    pub summary: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub keywords: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub labels: Vec<String>,
    #[serde(rename = "ts")]
    pub timestamp: i64,
    pub score: f64,
}

// ---------------------------------------------------------------------------
// CompressPolicy
// ---------------------------------------------------------------------------

/// Controls when auto-compression triggers during [Conversation::append].
#[derive(Debug, Clone, Copy)]
pub struct CompressPolicy {
    pub max_chars: usize,
    pub max_messages: usize,
}

impl Default for CompressPolicy {
    fn default() -> Self {
        Self {
            max_chars: 32000,
            max_messages: 30,
        }
    }
}

impl CompressPolicy {
    pub fn disabled() -> Self {
        Self { max_chars: 0, max_messages: 0 }
    }

    pub fn should_compress(&self, chars: usize, msgs: usize) -> bool {
        if self.max_chars == 0 && self.max_messages == 0 {
            return false;
        }
        if self.max_chars > 0 && chars >= self.max_chars {
            return true;
        }
        if self.max_messages > 0 && msgs >= self.max_messages {
            return true;
        }
        false
    }
}

// ---------------------------------------------------------------------------
// Compressor trait
// ---------------------------------------------------------------------------

/// Implemented by the agent runtime to drive message compression
/// and segment compaction. Memory calls these when conversations exceed
/// thresholds or when bucket compaction is triggered.
#[async_trait::async_trait]
pub trait Compressor: Send + Sync {
    /// Compress conversation messages into memory segments.
    async fn compress_messages(&self, messages: &[Message]) -> Result<CompressResult, MemoryError>;

    /// Extract entity and relation updates from messages.
    async fn extract_entities(&self, messages: &[Message]) -> Result<EntityUpdate, MemoryError>;

    /// Compress multiple segment summaries into a single new segment.
    async fn compact_segments(&self, summaries: &[String]) -> Result<CompressResult, MemoryError>;
}

/// Output of compress_messages or compact_segments.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompressResult {
    pub segments: Vec<SegmentInput>,
    pub summary: String,
}

/// Input for creating a new memory segment.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SegmentInput {
    pub summary: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub keywords: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub labels: Vec<String>,
}

/// Entity and relation changes extracted from messages.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EntityUpdate {
    #[serde(default)]
    pub entities: Vec<EntityInput>,
    #[serde(default)]
    pub relations: Vec<RelationInput>,
}

/// An entity to create or merge attributes into.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EntityInput {
    pub label: String,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub attrs: HashMap<String, Value>,
}

/// A directed relation to add.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelationInput {
    pub from: String,
    pub to: String,
    pub rel_type: String,
}

// ---------------------------------------------------------------------------
// Monotonic timestamp
// ---------------------------------------------------------------------------

static LAST_NANO: AtomicI64 = AtomicI64::new(0);

/// Return a monotonically increasing Unix nanosecond timestamp.
/// Uses CAS loop identical to Go's nowNano().
pub fn now_nano() -> i64 {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("system clock before unix epoch")
        .as_nanos() as i64;
    loop {
        let old = LAST_NANO.load(Ordering::Relaxed);
        let next = if now > old { now } else { old + 1 };
        if LAST_NANO.compare_exchange_weak(old, next, Ordering::Release, Ordering::Relaxed).is_ok()
        {
            return next;
        }
    }
}
