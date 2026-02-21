pub mod compact;
pub mod conversation;
pub mod error;
pub mod host;
pub mod keys;
pub mod memory;
pub mod types;

pub use error::MemoryError;
pub use host::{Host, HostConfig};
pub use memory::Memory;
pub use conversation::Conversation;
pub use types::{
    CompressPolicy, CompressResult, Compressor, EntityInfo, EntityInput,
    EntityUpdate, Message, RecallQuery, RecallResult, RelationInput, Role,
    ScoredSegment, SegmentInput, messages_to_strings, now_nano,
};

#[cfg(test)]
mod tests;
