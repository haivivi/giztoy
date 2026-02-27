pub mod compact;
pub mod compressor;
pub mod conversation;
pub mod error;
pub mod host;
pub mod keys;
pub mod memory;
pub mod types;

pub use compressor::{LLMCompressor, LLMCompressorConfig, messages_to_strings};
pub use error::MemoryError;
pub use host::{Host, HostConfig};
pub use memory::Memory;
pub use conversation::Conversation;
pub use types::{
    CompressPolicy, CompressResult, Compressor, EntityInfo, EntityInput,
    EntityUpdate, Message, RecallQuery, RecallResult, RelationInput, Role,
    ScoredSegment, SegmentInput, now_nano,
};

#[cfg(test)]
mod tests;
