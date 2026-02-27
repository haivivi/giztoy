use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use openerp_kv::KVStore;

use giztoy_embed::Embedder;
use giztoy_recall::RecallIndex;
use giztoy_vecstore::VecIndex;

use crate::error::MemoryError;
use crate::keys::{host_meta_key, mem_prefix};
use crate::memory::Memory;
use crate::types::{CompressPolicy, Compressor};

/// Configures a [Host].
pub struct HostConfig {
    /// Shared KV store. Required.
    pub store: Arc<dyn KVStore>,

    /// Shared vector index. Optional.
    pub vec: Option<Arc<dyn VecIndex>>,

    /// Converts text to vectors. Optional.
    pub embedder: Option<Arc<dyn Embedder>>,

    /// Default compressor for LLM-based compression. Optional.
    pub compressor: Option<Arc<dyn Compressor>>,

    /// Controls when auto-compression triggers.
    pub compress_policy: CompressPolicy,

    /// KV key separator byte for the graph layer. Labels must not contain
    /// this character. Default ':' forbids colon in labels. Use '\x1F'
    /// (ASCII Unit Separator) for natural labels like "person:小明".
    pub separator: char,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct EmbedMeta {
    dim: usize,
}

/// Process-level entry point for the memory system.
/// Manages Memory instances for many personas, sharing a single KV store,
/// vector index, and embedder. Safe for concurrent use.
pub struct Host {
    store: Arc<dyn KVStore>,
    vec: Option<Arc<dyn VecIndex>>,
    embedder: Option<Arc<dyn Embedder>>,
    compressor: Option<Arc<dyn Compressor>>,
    policy: CompressPolicy,
    separator: char,

    memories: Mutex<HashMap<String, ()>>,
}

impl Host {
    /// Create a new Host. Validates embedding dimension consistency.
    pub fn new(cfg: HostConfig) -> Result<Self, MemoryError> {
        if let Some(ref emb) = cfg.embedder {
            check_embed_meta(cfg.store.as_ref(), emb.as_ref())?;
        }

        let mut policy = cfg.compress_policy;
        if policy.max_chars == 0 && policy.max_messages == 0 && cfg.compressor.is_some() {
            policy = CompressPolicy::default();
        }

        let separator = if cfg.separator == '\0' {
            giztoy_graph::DEFAULT_SEPARATOR
        } else {
            cfg.separator
        };

        Ok(Self {
            store: cfg.store,
            vec: cfg.vec,
            embedder: cfg.embedder,
            compressor: cfg.compressor,
            policy,
            separator,
            memories: Mutex::new(HashMap::new()),
        })
    }

    /// Open (or create) a Memory for a persona.
    pub fn open(&self, id: &str) -> Memory {
        let mut memories = self.memories.lock().expect("lock poisoned");
        memories.entry(id.to_string()).or_insert(());

        let prefix = mem_prefix(id);

        let index = RecallIndex::with_separator(
            Box::new(SharedStore(Arc::clone(&self.store))),
            Box::new(SharedStore(Arc::clone(&self.store))),
            self.embedder.as_ref().map(|e| -> Box<dyn Embedder> {
                Box::new(SharedEmbedder(Arc::clone(e)))
            }),
            self.vec.as_ref().map(|v| -> Box<dyn VecIndex> {
                Box::new(SharedVecIndex(Arc::clone(v)))
            }),
            prefix,
            self.separator,
        );

        Memory::new(
            id.to_string(),
            Arc::clone(&self.store),
            index,
            self.compressor.as_ref().map(Arc::clone),
            self.policy,
        )
    }

    /// List all persona IDs that have been opened.
    pub fn list(&self) -> Vec<String> {
        let memories = self.memories.lock().expect("lock poisoned");
        let mut ids: Vec<String> = memories.keys().cloned().collect();
        ids.sort();
        ids
    }

    /// Delete all data for a persona. Safe to call for non-existent IDs.
    pub fn delete(&self, id: &str) -> Result<(), MemoryError> {
        let persona_prefix = mem_prefix(id);
        let sep = self.separator;

        // Delete known per-persona namespaces with scoped prefix scans.
        // conv/seg/sid use ':' as namespace delimiter (fixed in key format).
        // graph uses the configurable separator, so we append it explicitly
        // to avoid matching unrelated personas (e.g. "a:goals" when deleting "a").
        let ns_prefixes = vec![
            format!("{persona_prefix}:conv:"),
            format!("{persona_prefix}:seg:"),
            format!("{persona_prefix}:sid:"),
            format!("{persona_prefix}:g{sep}"),
        ];

        for scoped in &ns_prefixes {
            let entries = self.store.scan(scoped)?;
            if !entries.is_empty() {
                let keys: Vec<&str> = entries.iter().map(|(k, _)| k.as_str()).collect();
                self.store.batch_delete(&keys)?;
            }
        }

        let mut memories = self.memories.lock().expect("lock poisoned");
        memories.remove(id);
        Ok(())
    }
}

fn check_embed_meta(store: &dyn KVStore, emb: &dyn Embedder) -> Result<(), MemoryError> {
    let key = host_meta_key("embed");
    let current = EmbedMeta { dim: emb.dimension() };

    match store.get(&key)? {
        None => {
            let data = serde_json::to_vec(&current)
                .map_err(|e| MemoryError::Serialization(e.to_string()))?;
            store.set(&key, &data)?;
            Ok(())
        }
        Some(data) => {
            let stored: EmbedMeta = serde_json::from_slice(&data)
                .map_err(|e| MemoryError::Serialization(e.to_string()))?;
            if stored.dim != current.dim {
                return Err(MemoryError::EmbedDimensionMismatch {
                    model: String::new(),
                    stored: stored.dim,
                    current: current.dim,
                });
            }
            Ok(())
        }
    }
}

// ---------------------------------------------------------------------------
// Arc wrappers: delegate trait methods to the inner Arc-shared object
// ---------------------------------------------------------------------------

struct SharedStore(Arc<dyn KVStore>);

impl KVStore for SharedStore {
    fn get(&self, key: &str) -> Result<Option<Vec<u8>>, openerp_kv::KVError> {
        self.0.get(key)
    }
    fn set(&self, key: &str, value: &[u8]) -> Result<(), openerp_kv::KVError> {
        self.0.set(key, value)
    }
    fn delete(&self, key: &str) -> Result<(), openerp_kv::KVError> {
        self.0.delete(key)
    }
    fn scan(&self, prefix: &str) -> Result<Vec<(String, Vec<u8>)>, openerp_kv::KVError> {
        self.0.scan(prefix)
    }
    fn batch_set(&self, entries: &[(&str, &[u8])]) -> Result<(), openerp_kv::KVError> {
        self.0.batch_set(entries)
    }
    fn batch_delete(&self, keys: &[&str]) -> Result<(), openerp_kv::KVError> {
        self.0.batch_delete(keys)
    }
    fn is_readonly(&self, key: &str) -> bool {
        self.0.is_readonly(key)
    }
}

struct SharedEmbedder(Arc<dyn Embedder>);

#[async_trait::async_trait]
impl Embedder for SharedEmbedder {
    async fn embed(&self, text: &str) -> Result<Vec<f32>, giztoy_embed::EmbedError> {
        self.0.embed(text).await
    }
    async fn embed_batch(&self, texts: &[&str]) -> Result<Vec<Vec<f32>>, giztoy_embed::EmbedError> {
        self.0.embed_batch(texts).await
    }
    fn dimension(&self) -> usize {
        self.0.dimension()
    }
}

struct SharedVecIndex(Arc<dyn VecIndex>);

impl VecIndex for SharedVecIndex {
    fn insert(&self, id: &str, vector: &[f32]) -> Result<(), giztoy_vecstore::VecError> {
        self.0.insert(id, vector)
    }
    fn batch_insert(&self, ids: &[&str], vectors: &[&[f32]]) -> Result<(), giztoy_vecstore::VecError> {
        self.0.batch_insert(ids, vectors)
    }
    fn search(&self, query: &[f32], top_k: usize) -> Result<Vec<giztoy_vecstore::Match>, giztoy_vecstore::VecError> {
        self.0.search(query, top_k)
    }
    fn delete(&self, id: &str) -> Result<(), giztoy_vecstore::VecError> {
        self.0.delete(id)
    }
    fn len(&self) -> usize {
        self.0.len()
    }
    fn flush(&self) -> Result<(), giztoy_vecstore::VecError> {
        self.0.flush()
    }
}
