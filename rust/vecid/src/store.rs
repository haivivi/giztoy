use std::sync::Mutex;

use crate::VecIdError;

/// Persists raw embeddings for re-clustering.
///
/// Implementations must be safe for concurrent use.
/// Use [`MemoryStore`] for in-memory storage (testing/ephemeral).
pub trait VecIdStore: Send + Sync {
    /// Stores an embedding. Returns a unique sequence ID.
    fn append(&self, emb: &[f32]) -> Result<u64, VecIdError>;

    /// Returns all stored embeddings in insertion order.
    fn all(&self) -> Result<Vec<Vec<f32>>, VecIdError>;

    /// Returns the count of stored embeddings.
    fn len(&self) -> Result<usize, VecIdError>;

    /// Removes all stored embeddings.
    fn clear(&self) -> Result<(), VecIdError>;
}

/// In-memory [`VecIdStore`] implementation.
/// Data is lost on restart. Suitable for testing or ephemeral use.
pub struct MemoryStore {
    inner: Mutex<MemoryStoreInner>,
}

struct MemoryStoreInner {
    data: Vec<Vec<f32>>,
    seq: u64,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self {
            inner: Mutex::new(MemoryStoreInner {
                data: Vec::new(),
                seq: 0,
            }),
        }
    }
}

impl Default for MemoryStore {
    fn default() -> Self {
        Self::new()
    }
}

impl VecIdStore for MemoryStore {
    fn append(&self, emb: &[f32]) -> Result<u64, VecIdError> {
        let mut inner = self.inner.lock().unwrap();
        inner.data.push(emb.to_vec());
        inner.seq += 1;
        Ok(inner.seq)
    }

    fn all(&self) -> Result<Vec<Vec<f32>>, VecIdError> {
        let inner = self.inner.lock().unwrap();
        Ok(inner.data.clone())
    }

    fn len(&self) -> Result<usize, VecIdError> {
        let inner = self.inner.lock().unwrap();
        Ok(inner.data.len())
    }

    fn clear(&self) -> Result<(), VecIdError> {
        let mut inner = self.inner.lock().unwrap();
        inner.data.clear();
        inner.seq = 0;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn memory_store_append_and_all() {
        let store = MemoryStore::new();
        let seq1 = store.append(&[1.0, 2.0, 3.0]).unwrap();
        let seq2 = store.append(&[4.0, 5.0, 6.0]).unwrap();
        assert_eq!(seq1, 1);
        assert_eq!(seq2, 2);
        assert_eq!(store.len().unwrap(), 2);

        let all = store.all().unwrap();
        assert_eq!(all.len(), 2);
        assert_eq!(all[0], vec![1.0, 2.0, 3.0]);
        assert_eq!(all[1], vec![4.0, 5.0, 6.0]);
    }

    #[test]
    fn memory_store_clear() {
        let store = MemoryStore::new();
        store.append(&[1.0]).unwrap();
        store.append(&[2.0]).unwrap();
        assert_eq!(store.len().unwrap(), 2);

        store.clear().unwrap();
        assert_eq!(store.len().unwrap(), 0);
        assert!(store.all().unwrap().is_empty());

        // Sequence resets after clear.
        let seq = store.append(&[3.0]).unwrap();
        assert_eq!(seq, 1);
    }
}
