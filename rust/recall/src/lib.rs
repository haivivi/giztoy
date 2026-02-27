pub mod error;
pub mod index;
pub mod keys;
pub mod search;
pub mod segment;
pub mod types;

pub use error::RecallError;
pub use index::RecallIndex;
pub use types::{
    Bucket, Query, ScoredSegment, SearchQuery, SearchResult, Segment,
    all_buckets, bucket_1h, bucket_1d, bucket_1w, bucket_1m, bucket_3m,
    bucket_6m, bucket_1y, bucket_lt, bucket_for_span, compactable_buckets,
};
