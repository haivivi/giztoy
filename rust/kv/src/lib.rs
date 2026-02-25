//! Key-value store interface and implementations.
//!
//! Provides a trait-based KV store interface with an in-memory implementation
//! for testing and a redb-based implementation for persistence.

pub mod memory;
pub mod redb;

use std::fmt;
use thiserror::Error;

/// Errors that can occur in KV store operations.
#[derive(Error, Debug)]
pub enum KVError {
    #[error("kv: not found")]
    NotFound,
    
    #[error("kv: storage error: {0}")]
    Storage(String),
    
    #[error("kv: serialization error: {0}")]
    Serialization(String),
}

/// Result type for KV operations.
pub type KVResult<T> = Result<T, KVError>;

/// Key-value store trait.
///
/// This trait provides basic operations for storing and retrieving
/// key-value pairs with string keys and byte values.
pub trait KVStore: Send + Sync {
    /// Get a value by key.
    fn get(&self, key: &str) -> KVResult<Option<Vec<u8>>>;
    
    /// Set a key-value pair.
    fn set(&self, key: &str, value: &[u8]) -> KVResult<()>;
    
    /// Delete a key.
    fn delete(&self, key: &str) -> KVResult<()>;
    
    /// Scan for keys with a given prefix.
    fn scan(&self, prefix: &str) -> KVResult<Vec<(String, Vec<u8>)>>;
    
    /// Batch set multiple key-value pairs.
    fn batch_set(&self, entries: &[(&str, &[u8])]) -> KVResult<()>;
    
    /// Batch delete multiple keys.
    fn batch_delete(&self, keys: &[&str]) -> KVResult<()>;
}

impl fmt::Debug for dyn KVStore {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "KVStore {{ ... }}")
    }
}

/// A boxed KV store for use in trait objects.
pub type BoxedKVStore = Box<dyn KVStore>;

// Re-export the implementations
pub use memory::MemoryStore;
pub use redb::RedbStore;
