//! In-memory key-value store implementation for testing.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use crate::{KVError, KVResult, KVStore};

/// An in-memory key-value store backed by a HashMap.
#[derive(Clone)]
pub struct MemoryStore {
    data: Arc<Mutex<HashMap<String, Vec<u8>>>>,
}

impl MemoryStore {
    /// Create a new empty memory store.
    pub fn new() -> Self {
        Self {
            data: Arc::new(Mutex::new(HashMap::new())),
        }
    }
}

impl Default for MemoryStore {
    fn default() -> Self {
        Self::new()
    }
}

impl KVStore for MemoryStore {
    fn get(&self, key: &str) -> KVResult<Option<Vec<u8>>> {
        let data = self
            .data
            .lock()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        Ok(data.get(key).cloned())
    }

    fn set(&self, key: &str, value: &[u8]) -> KVResult<()> {
        let mut data = self
            .data
            .lock()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        data.insert(key.to_string(), value.to_vec());
        Ok(())
    }

    fn delete(&self, key: &str) -> KVResult<()> {
        let mut data = self
            .data
            .lock()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        data.remove(key);
        Ok(())
    }

    fn scan(&self, prefix: &str) -> KVResult<Vec<(String, Vec<u8>)>> {
        let data = self
            .data
            .lock()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        let mut results: Vec<(String, Vec<u8>)> = data
            .iter()
            .filter(|(k, _)| k.starts_with(prefix))
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect();
        results.sort_by(|a, b| a.0.cmp(&b.0));
        Ok(results)
    }

    fn batch_set(&self, entries: &[(&str, &[u8])]) -> KVResult<()> {
        let mut data = self
            .data
            .lock()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        for (key, value) in entries {
            data.insert(key.to_string(), value.to_vec());
        }
        Ok(())
    }

    fn batch_delete(&self, keys: &[&str]) -> KVResult<()> {
        let mut data = self
            .data
            .lock()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        for key in keys {
            data.remove(*key);
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_operations() {
        let store = MemoryStore::new();

        // Set and get
        store.set("key1", b"value1").unwrap();
        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));

        // Non-existent key
        assert_eq!(store.get("nonexistent").unwrap(), None);

        // Delete
        store.delete("key1").unwrap();
        assert_eq!(store.get("key1").unwrap(), None);
    }

    #[test]
    fn test_scan() {
        let store = MemoryStore::new();
        store.set("prefix:a", b"1").unwrap();
        store.set("prefix:b", b"2").unwrap();
        store.set("other:c", b"3").unwrap();

        let results = store.scan("prefix:").unwrap();
        assert_eq!(results.len(), 2);
    }

    #[test]
    fn test_batch_operations() {
        let store = MemoryStore::new();

        store
            .batch_set(&[("key1", b"value1"), ("key2", b"value2")])
            .unwrap();

        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));
        assert_eq!(store.get("key2").unwrap(), Some(b"value2".to_vec()));

        store.batch_delete(&["key1", "key2"]).unwrap();
        assert_eq!(store.get("key1").unwrap(), None);
        assert_eq!(store.get("key2").unwrap(), None);
    }
}
