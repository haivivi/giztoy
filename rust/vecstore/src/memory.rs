use std::collections::HashMap;
use std::sync::RwLock;

use crate::cosine::cosine_distance;
use crate::error::VecError;
use crate::vecstore::{Match, VecIndex};

/// Memory is an in-memory VecIndex using brute-force cosine distance.
/// Intended for testing and small-scale use (< 1000 vectors).
pub struct MemoryIndex {
    vectors: RwLock<HashMap<String, Vec<f32>>>,
}

impl MemoryIndex {
    pub fn new() -> Self {
        Self {
            vectors: RwLock::new(HashMap::new()),
        }
    }
}

impl Default for MemoryIndex {
    fn default() -> Self {
        Self::new()
    }
}

impl VecIndex for MemoryIndex {
    fn insert(&self, id: &str, vector: &[f32]) -> Result<(), VecError> {
        let mut vecs = self.vectors.write().unwrap();
        vecs.insert(id.to_string(), vector.to_vec());
        Ok(())
    }

    fn batch_insert(&self, ids: &[&str], vectors: &[&[f32]]) -> Result<(), VecError> {
        if ids.len() != vectors.len() {
            return Err(VecError::BatchLengthMismatch {
                ids: ids.len(),
                vectors: vectors.len(),
            });
        }
        let mut vecs = self.vectors.write().unwrap();
        for (id, vec) in ids.iter().zip(vectors.iter()) {
            vecs.insert(id.to_string(), vec.to_vec());
        }
        Ok(())
    }

    fn search(&self, query: &[f32], top_k: usize) -> Result<Vec<Match>, VecError> {
        let vecs = self.vectors.read().unwrap();
        if vecs.is_empty() || top_k == 0 {
            return Ok(vec![]);
        }

        let mut results: Vec<(String, f32)> = vecs
            .iter()
            .map(|(id, vec)| (id.clone(), cosine_distance(query, vec)))
            .collect();

        results.sort_by(|a, b| a.1.partial_cmp(&b.1).unwrap_or(std::cmp::Ordering::Equal));

        if results.len() > top_k {
            results.truncate(top_k);
        }

        Ok(results
            .into_iter()
            .map(|(id, distance)| Match { id, distance })
            .collect())
    }

    fn delete(&self, id: &str) -> Result<(), VecError> {
        let mut vecs = self.vectors.write().unwrap();
        vecs.remove(id);
        Ok(())
    }

    fn len(&self) -> usize {
        self.vectors.read().unwrap().len()
    }

    fn flush(&self) -> Result<(), VecError> {
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_insert_and_search() {
        let idx = MemoryIndex::new();
        idx.insert("a", &[1.0, 0.0, 0.0, 0.0]).unwrap();
        idx.insert("b", &[0.0, 1.0, 0.0, 0.0]).unwrap();
        idx.insert("c", &[0.9, 0.1, 0.0, 0.0]).unwrap();

        let matches = idx.search(&[1.0, 0.0, 0.0, 0.0], 2).unwrap();
        assert_eq!(matches.len(), 2);
        assert_eq!(matches[0].id, "a");
        assert_eq!(matches[1].id, "c");
    }

    #[test]
    fn test_batch_insert() {
        let idx = MemoryIndex::new();
        idx.batch_insert(
            &["a", "b", "c"],
            &[&[1.0, 0.0, 0.0], &[0.0, 1.0, 0.0], &[0.0, 0.0, 1.0]],
        )
        .unwrap();
        assert_eq!(idx.len(), 3);

        let matches = idx.search(&[1.0, 0.0, 0.0], 1).unwrap();
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].id, "a");
    }

    #[test]
    fn test_batch_insert_mismatch() {
        let idx = MemoryIndex::new();
        assert!(idx
            .batch_insert(&["a", "b"], &[&[1.0, 0.0]])
            .is_err());
    }

    #[test]
    fn test_delete() {
        let idx = MemoryIndex::new();
        idx.insert("a", &[1.0, 0.0]).unwrap();
        assert_eq!(idx.len(), 1);
        idx.delete("a").unwrap();
        assert_eq!(idx.len(), 0);
        idx.delete("nonexistent").unwrap();
    }

    #[test]
    fn test_search_empty() {
        let idx = MemoryIndex::new();
        let matches = idx.search(&[1.0, 0.0, 0.0], 5).unwrap();
        assert!(matches.is_empty());
    }

    #[test]
    fn test_flush() {
        assert!(MemoryIndex::new().flush().is_ok());
    }
}
