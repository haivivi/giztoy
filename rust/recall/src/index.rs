use openerp_kv::KVStore;

use giztoy_embed::Embedder;
use giztoy_graph::{Graph, KVGraph};
use giztoy_vecstore::VecIndex;

use crate::keys::graph_prefix;

/// RecallIndex is a single search space combining segment storage,
/// an entity-relation graph, and multi-signal search.
pub struct RecallIndex {
    pub(crate) store: Box<dyn KVStore>,
    pub(crate) embedder: Option<Box<dyn Embedder>>,
    pub(crate) vec: Option<Box<dyn VecIndex>>,
    pub(crate) graph: KVGraph,
    pub(crate) prefix: String,
}

impl RecallIndex {
    /// Create a RecallIndex. The graph is constructed from the same KV store.
    ///
    /// Since Rust ownership prevents sharing a single Box<dyn KVStore> between
    /// the index and the graph, the caller provides two store handles that
    /// point to the same underlying database. Both RedbStore instances opened
    /// on the same path share the same underlying database via redb's internal
    /// locking.
    pub fn new(
        store: Box<dyn KVStore>,
        graph_store: Box<dyn KVStore>,
        embedder: Option<Box<dyn Embedder>>,
        vec_index: Option<Box<dyn VecIndex>>,
        prefix: String,
    ) -> Self {
        let gprefix = graph_prefix(&prefix);
        Self {
            store,
            embedder,
            vec: vec_index,
            graph: KVGraph::new(graph_store, &gprefix),
            prefix,
        }
    }

    /// Return a reference to the entity-relation graph.
    pub fn graph(&self) -> &dyn Graph {
        &self.graph
    }
}
