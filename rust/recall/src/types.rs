use std::time::Duration;

use serde::{Deserialize, Serialize};

/// Bucket identifies a time-granularity bucket for segment storage.
///
/// Segments are organized into buckets based on the time span they cover.
/// When a bucket exceeds its capacity, the oldest segments are compacted
/// into a coarser bucket.
///
/// The hierarchy: 1h → 1d → 1w → 1m → 3m → 6m → 1y → lt
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub struct Bucket(pub String);

impl Bucket {
    pub const BUCKET_1H: &str = "1h";
    pub const BUCKET_1D: &str = "1d";
    pub const BUCKET_1W: &str = "1w";
    pub const BUCKET_1M: &str = "1m";
    pub const BUCKET_3M: &str = "3m";
    pub const BUCKET_6M: &str = "6m";
    pub const BUCKET_1Y: &str = "1y";
    pub const BUCKET_LT: &str = "lt";

    pub fn as_str(&self) -> &str {
        &self.0
    }

    pub fn duration(&self) -> Duration {
        match self.0.as_str() {
            Self::BUCKET_1H => Duration::from_secs(3600),
            Self::BUCKET_1D => Duration::from_secs(86400),
            Self::BUCKET_1W => Duration::from_secs(7 * 86400),
            Self::BUCKET_1M => Duration::from_secs(30 * 86400),
            Self::BUCKET_3M => Duration::from_secs(90 * 86400),
            Self::BUCKET_6M => Duration::from_secs(180 * 86400),
            Self::BUCKET_1Y => Duration::from_secs(365 * 86400),
            _ => Duration::ZERO, // lt or unknown
        }
    }
}

impl std::fmt::Display for Bucket {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

pub fn bucket_1h() -> Bucket { Bucket(Bucket::BUCKET_1H.into()) }
pub fn bucket_1d() -> Bucket { Bucket(Bucket::BUCKET_1D.into()) }
pub fn bucket_1w() -> Bucket { Bucket(Bucket::BUCKET_1W.into()) }
pub fn bucket_1m() -> Bucket { Bucket(Bucket::BUCKET_1M.into()) }
pub fn bucket_3m() -> Bucket { Bucket(Bucket::BUCKET_3M.into()) }
pub fn bucket_6m() -> Bucket { Bucket(Bucket::BUCKET_6M.into()) }
pub fn bucket_1y() -> Bucket { Bucket(Bucket::BUCKET_1Y.into()) }
pub fn bucket_lt() -> Bucket { Bucket(Bucket::BUCKET_LT.into()) }

/// All buckets in order from finest to coarsest.
pub fn all_buckets() -> Vec<Bucket> {
    vec![bucket_1h(), bucket_1d(), bucket_1w(), bucket_1m(), bucket_3m(), bucket_6m(), bucket_1y(), bucket_lt()]
}

/// All buckets that can be compacted (excludes lt).
pub fn compactable_buckets() -> Vec<Bucket> {
    vec![bucket_1h(), bucket_1d(), bucket_1w(), bucket_1m(), bucket_3m(), bucket_6m(), bucket_1y()]
}

/// Return the smallest bucket whose duration covers the given time span.
pub fn bucket_for_span(span: Duration) -> Bucket {
    for b in all_buckets() {
        let d = b.duration();
        if d.is_zero() || span <= d {
            return b;
        }
    }
    bucket_lt()
}

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

    /// Time-granularity bucket. Defaults to "1h" for fresh segments.
    #[serde(rename = "bucket", default = "default_bucket", skip_serializing_if = "String::is_empty")]
    pub bucket: String,
}

fn default_bucket() -> String { Bucket::BUCKET_1H.into() }

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
