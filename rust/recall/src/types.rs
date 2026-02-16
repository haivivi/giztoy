use serde::{Deserialize, Serialize};

/// Segment is a memory fragment stored in the index.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Segment {
    /// Unique identifier for this segment.
    #[serde(rename = "id")]
    pub id: String,

    /// Human-readable text content.
    #[serde(rename = "summary")]
    pub summary: String,

    /// Terms extracted for keyword matching.
    #[serde(rename = "keywords", default, skip_serializing_if = "Vec::is_empty")]
    pub keywords: Vec<String>,

    /// Entity references (e.g. "person:Alice", "topic:dinosaurs").
    #[serde(rename = "labels", default, skip_serializing_if = "Vec::is_empty")]
    pub labels: Vec<String>,

    /// Unix timestamp in nanoseconds.
    #[serde(rename = "ts")]
    pub timestamp: i64,
}

/// SearchQuery specifies parameters for segment search.
pub struct SearchQuery {
    /// Query text for vector embedding and keyword matching.
    pub text: String,

    /// Filter segments by label overlap.
    pub labels: Vec<String>,

    /// Maximum number of results. Default: 10.
    pub limit: usize,

    /// Filter segments created at or after this time (unix nanoseconds).
    /// 0 means no lower bound.
    pub after_ns: i64,

    /// Filter segments created before this time (unix nanoseconds).
    /// 0 means no upper bound.
    pub before_ns: i64,
}

/// ScoredSegment pairs a segment with its relevance score.
#[derive(Debug, Clone)]
pub struct ScoredSegment {
    pub segment: Segment,
    pub score: f64,
}

/// Query specifies parameters for the high-level combined search.
pub struct Query {
    /// Seed entity labels for graph expansion.
    pub labels: Vec<String>,

    /// Search text for vector + keyword matching.
    pub text: String,

    /// Max graph traversal hops. Default: 2.
    pub hops: usize,

    /// Max results. Default: 10.
    pub limit: usize,
}

/// Result holds the output of a combined search.
pub struct SearchResult {
    /// Matching segments, scored and sorted by relevance.
    pub segments: Vec<ScoredSegment>,

    /// Entity labels after graph expansion (includes seeds).
    pub expanded: Vec<String>,
}
