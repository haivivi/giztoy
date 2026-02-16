use std::fmt;
use std::sync::RwLock;

use crate::dbscan::{cosine_sim, dbscan, l2_norm};
use crate::store::{MemoryStore, VecIdStore};

/// Controls registry behavior.
pub struct Config {
    /// Embedding dimension (e.g. 512 for voice, 1536 for text).
    pub dim: usize,

    /// Minimum cosine similarity to match a bucket.
    /// Lower = more lenient (more merges), higher = stricter (more unknowns).
    /// Default: 0.5.
    pub threshold: f32,

    /// Minimum number of samples to form a cluster in DBSCAN.
    /// Default: 2.
    pub min_samples: usize,

    /// Prepended to generated IDs (e.g. "speaker" -> "speaker:001").
    pub prefix: String,
}

impl Config {
    fn with_defaults(mut self) -> Self {
        if self.threshold == 0.0 {
            self.threshold = 0.5;
        }
        if self.min_samples == 0 {
            self.min_samples = 2;
        }
        self
    }
}

/// Represents a cluster of similar embeddings.
#[derive(Clone)]
pub struct Bucket {
    /// Stable identifier (e.g. "speaker:001").
    pub id: String,

    /// L2-normalized mean embedding of this cluster.
    pub centroid: Vec<f32>,

    /// Number of embeddings in this cluster.
    pub count: usize,
}

impl fmt::Debug for Bucket {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Bucket")
            .field("id", &self.id)
            .field("count", &self.count)
            .field("centroid_len", &self.centroid.len())
            .finish()
    }
}

struct RegistryInner {
    cfg: Config,
    buckets: Vec<Bucket>,
    next_id: usize,
}

/// Assigns stable IDs to embedding vectors.
///
/// Thread-safe: all methods can be called concurrently.
pub struct Registry {
    inner: RwLock<RegistryInner>,
    store: Box<dyn VecIdStore>,
}

impl Registry {
    /// Creates a new Registry. Panics if `cfg.dim` is 0.
    pub fn new(cfg: Config, store: Box<dyn VecIdStore>) -> Self {
        assert!(cfg.dim > 0, "vecid: Config.dim must be positive");
        let cfg = cfg.with_defaults();
        Self {
            inner: RwLock::new(RegistryInner {
                cfg,
                buckets: Vec::new(),
                next_id: 0,
            }),
            store,
        }
    }

    /// Creates a new Registry with a default in-memory store.
    pub fn with_memory_store(cfg: Config) -> Self {
        Self::new(cfg, Box::new(MemoryStore::new()))
    }

    /// Returns the best matching bucket ID for the embedding.
    /// The embedding is always stored for future re-clustering.
    ///
    /// Returns `(id, confidence, matched)`:
    /// - `id`: the matched bucket ID, or empty string if no match
    /// - `confidence`: cosine similarity to the matched bucket (0 if unmatched)
    /// - `matched`: true if a bucket was found above threshold
    pub fn identify(&self, emb: &[f32]) -> (String, f32, bool) {
        // Always store for future re-clustering.
        let _ = self.store.append(emb);

        let inner = self.inner.read().unwrap();

        if inner.buckets.is_empty() {
            return (String::new(), 0.0, false);
        }

        let mut best_sim: f32 = -1.0;
        let mut best_idx: Option<usize> = None;
        for (i, b) in inner.buckets.iter().enumerate() {
            let sim = cosine_sim(emb, &b.centroid);
            if sim > best_sim {
                best_sim = sim;
                best_idx = Some(i);
            }
        }

        if let Some(idx) = best_idx {
            if best_sim >= inner.cfg.threshold {
                return (inner.buckets[idx].id.clone(), best_sim, true);
            }
        }
        (String::new(), 0.0, false)
    }

    /// Re-runs DBSCAN on all stored embeddings.
    /// Updates bucket centroids and tries to preserve existing IDs.
    /// Returns the number of clusters found.
    pub fn recluster(&self) -> usize {
        let embeddings = match self.store.all() {
            Ok(e) => e,
            Err(_) => return 0,
        };
        if embeddings.is_empty() {
            return 0;
        }

        // Snapshot config under read lock.
        let (threshold, min_samples, dim) = {
            let inner = self.inner.read().unwrap();
            (inner.cfg.threshold, inner.cfg.min_samples, inner.cfg.dim)
        };

        // L2-normalize all embeddings for cosine distance.
        let normed: Vec<Vec<f32>> = embeddings
            .iter()
            .map(|emb| {
                let mut cp = emb.clone();
                l2_norm(&mut cp);
                cp
            })
            .collect();

        // DBSCAN: eps = 1 - threshold (cosine distance).
        let eps = 1.0 - threshold;
        let refs: Vec<&[f32]> = normed.iter().map(|v| v.as_slice()).collect();
        let labels = dbscan(&refs, eps, min_samples);

        // Find max positive cluster label (ignoring noise = -1).
        let max_label = labels.iter().copied().filter(|&l| l > 0).max().unwrap_or(0);

        // Compute centroids for each cluster.
        let mut new_buckets: Vec<Bucket> = Vec::with_capacity(max_label as usize);
        for c in 1..=max_label {
            let mut centroid = vec![0.0f32; dim];
            let mut count = 0usize;
            for (i, &l) in labels.iter().enumerate() {
                if l == c {
                    for (d, val) in centroid.iter_mut().enumerate() {
                        if d < normed[i].len() {
                            *val += normed[i][d];
                        }
                    }
                    count += 1;
                }
            }
            if count == 0 {
                continue;
            }
            let n = count as f32;
            for val in centroid.iter_mut() {
                *val /= n;
            }
            l2_norm(&mut centroid);
            new_buckets.push(Bucket {
                id: String::new(),
                centroid,
                count,
            });
        }

        // Assign IDs: match new buckets to old ones by centroid similarity.
        let mut inner = self.inner.write().unwrap();

        // Snapshot old buckets before mutation.
        let old_buckets: Vec<Bucket> = inner.buckets.clone();
        let mut used_old_ids: Vec<bool> = vec![false; old_buckets.len()];

        for new_bucket in &mut new_buckets {
            let mut best_sim: f32 = -1.0;
            let mut best_old_idx: Option<usize> = None;
            for (j, old) in old_buckets.iter().enumerate() {
                if used_old_ids[j] {
                    continue;
                }
                let sim = cosine_sim(&new_bucket.centroid, &old.centroid);
                if sim > best_sim {
                    best_sim = sim;
                    best_old_idx = Some(j);
                }
            }
            if let Some(j) = best_old_idx {
                if best_sim >= threshold {
                    new_bucket.id = old_buckets[j].id.clone();
                    used_old_ids[j] = true;
                    continue;
                }
            }
            // Assign new ID.
            inner.next_id += 1;
            new_bucket.id = if inner.cfg.prefix.is_empty() {
                format!("{:03}", inner.next_id)
            } else {
                format!("{}:{:03}", inner.cfg.prefix, inner.next_id)
            };
        }

        let n = new_buckets.len();
        inner.buckets = new_buckets;
        n
    }

    /// Returns all current buckets.
    pub fn buckets(&self) -> Vec<Bucket> {
        let inner = self.inner.read().unwrap();
        inner.buckets.clone()
    }

    /// Returns the bucket for a given ID, or None if not found.
    pub fn bucket_of(&self, id: &str) -> Option<Bucket> {
        let inner = self.inner.read().unwrap();
        inner.buckets.iter().find(|b| b.id == id).cloned()
    }

    /// Adjusts matching strictness at runtime.
    pub fn set_threshold(&self, t: f32) {
        let mut inner = self.inner.write().unwrap();
        inner.cfg.threshold = t;
    }

    /// Returns the number of stored embeddings.
    pub fn len(&self) -> usize {
        self.store.len().unwrap_or(0)
    }

    /// Returns true if no embeddings are stored.
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Clears all stored embeddings and buckets.
    pub fn reset(&self) {
        let mut inner = self.inner.write().unwrap();
        let _ = self.store.clear();
        inner.buckets.clear();
        inner.next_id = 0;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn identify_before_recluster() {
        let reg = Registry::with_memory_store(Config {
            dim: 4,
            threshold: 0.5,
            min_samples: 2,
            prefix: "test".into(),
        });

        let emb = vec![1.0, 0.0, 0.0, 0.0];
        let (id, _conf, matched) = reg.identify(&emb);
        assert!(!matched, "should not match before recluster");
        assert!(id.is_empty());
        assert_eq!(reg.len(), 1);
    }

    #[test]
    fn recluster_creates_buckets() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.5,
            min_samples: 2,
            prefix: "speaker".into(),
        });

        // Add similar embeddings (cluster A).
        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.99, 0.1, 0.0]);
        reg.identify(&[0.98, 0.15, 0.0]);

        // Add similar embeddings (cluster B).
        reg.identify(&[0.0, 1.0, 0.0]);
        reg.identify(&[0.1, 0.99, 0.0]);
        reg.identify(&[0.15, 0.98, 0.0]);

        let n = reg.recluster();
        assert_eq!(n, 2, "should find 2 clusters");

        let buckets = reg.buckets();
        assert_eq!(buckets.len(), 2);
        assert!(buckets[0].id.starts_with("speaker:"));
        assert!(buckets[1].id.starts_with("speaker:"));
        assert_ne!(buckets[0].id, buckets[1].id);
    }

    #[test]
    fn identify_after_recluster() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.3,
            min_samples: 2,
            prefix: "s".into(),
        });

        // Cluster A.
        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.99, 0.1, 0.0]);

        // Cluster B.
        reg.identify(&[0.0, 1.0, 0.0]);
        reg.identify(&[0.1, 0.99, 0.0]);

        reg.recluster();

        // New embedding close to cluster A.
        let (id, conf, matched) = reg.identify(&[0.97, 0.2, 0.0]);
        assert!(matched, "should match cluster A");
        assert!(!id.is_empty());
        assert!(conf > 0.3);
    }

    #[test]
    fn recluster_preserves_ids() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.3,
            min_samples: 2,
            prefix: "v".into(),
        });

        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.99, 0.1, 0.0]);
        reg.identify(&[0.0, 1.0, 0.0]);
        reg.identify(&[0.1, 0.99, 0.0]);

        reg.recluster();
        let ids1: Vec<String> = reg.buckets().iter().map(|b| b.id.clone()).collect();

        // Add more points to existing clusters and recluster.
        reg.identify(&[0.98, 0.15, 0.0]);
        reg.identify(&[0.15, 0.98, 0.0]);

        reg.recluster();
        let ids2: Vec<String> = reg.buckets().iter().map(|b| b.id.clone()).collect();

        // IDs should be preserved since centroids didn't drift much.
        assert_eq!(ids1.len(), ids2.len());
        for id in &ids1 {
            assert!(ids2.contains(id), "ID {id} should be preserved after recluster");
        }
    }

    #[test]
    fn reset_clears_everything() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.5,
            min_samples: 2,
            prefix: "t".into(),
        });

        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.99, 0.1, 0.0]);
        reg.recluster();

        assert!(!reg.is_empty());
        assert!(!reg.buckets().is_empty());

        reg.reset();
        assert!(reg.is_empty());
        assert!(reg.buckets().is_empty());
    }

    #[test]
    fn set_threshold_runtime() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.99,
            min_samples: 2,
            prefix: "t".into(),
        });

        // Two somewhat similar vectors.
        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.9, 0.4, 0.0]);
        reg.recluster();

        // With very high threshold (0.99), eps = 0.01, so these are noise.
        let n1 = reg.buckets().len();
        assert_eq!(n1, 0, "high threshold should yield no clusters");

        // Lower threshold: eps = 0.7, now they cluster together.
        reg.set_threshold(0.3);
        reg.recluster();
        let n2 = reg.buckets().len();
        assert_eq!(n2, 1, "low threshold should merge into 1 cluster");
    }

    #[test]
    fn bucket_of_found_and_not_found() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.3,
            min_samples: 2,
            prefix: "s".into(),
        });

        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.99, 0.1, 0.0]);
        reg.recluster();

        let buckets = reg.buckets();
        assert!(!buckets.is_empty());

        let found = reg.bucket_of(&buckets[0].id);
        assert!(found.is_some());
        assert_eq!(found.unwrap().id, buckets[0].id);

        let not_found = reg.bucket_of("nonexistent:999");
        assert!(not_found.is_none());
    }

    #[test]
    fn id_format() {
        let reg = Registry::with_memory_store(Config {
            dim: 3,
            threshold: 0.3,
            min_samples: 2,
            prefix: "speaker".into(),
        });

        reg.identify(&[1.0, 0.0, 0.0]);
        reg.identify(&[0.99, 0.1, 0.0]);
        reg.recluster();

        let buckets = reg.buckets();
        assert!(!buckets.is_empty());
        // Format: "speaker:001"
        assert!(buckets[0].id.starts_with("speaker:"));
        let num_part = buckets[0].id.strip_prefix("speaker:").unwrap();
        assert_eq!(num_part.len(), 3, "should be zero-padded to 3 digits");
    }
}
