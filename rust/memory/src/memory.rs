use std::sync::Arc;

use giztoy_graph::{Entity, Graph, GraphError, Relation};
use giztoy_recall::{RecallIndex, Segment, bucket_1h};
use openerp_kv::KVStore;

use crate::conversation::Conversation;
use crate::error::MemoryError;
use crate::types::{
    CompressPolicy, Compressor, EntityInfo, EntityUpdate, RecallQuery,
    RecallResult, ScoredSegment, SegmentInput, now_nano,
};

/// Complete memory system for a single persona.
pub struct Memory {
    id: String,
    store: Arc<dyn KVStore>,
    index: RecallIndex,
    compressor: Option<Arc<dyn Compressor>>,
    policy: CompressPolicy,
}

impl Memory {
    pub(crate) fn new(
        id: String,
        store: Arc<dyn KVStore>,
        index: RecallIndex,
        compressor: Option<Arc<dyn Compressor>>,
        policy: CompressPolicy,
    ) -> Self {
        Self { id, store, index, compressor, policy }
    }

    pub fn id(&self) -> &str { &self.id }

    pub fn graph(&self) -> &dyn Graph { self.index.graph() }

    pub fn index(&self) -> &RecallIndex { &self.index }

    pub fn policy(&self) -> CompressPolicy { self.policy }

    pub fn has_compressor(&self) -> bool { self.compressor.is_some() }

    /// Open (or resume) a conversation session.
    pub fn open_conversation(&self, conv_id: &str, labels: &[String]) -> Conversation<'_> {
        Conversation::new(self, conv_id.to_string(), labels.to_vec())
    }

    /// Combined retrieval: graph expansion + multi-signal segment search +
    /// entity attribute lookup.
    pub async fn recall(&self, q: RecallQuery) -> Result<RecallResult, MemoryError> {
        let hops = if q.hops == 0 { 2 } else { q.hops };
        let limit = if q.limit == 0 { 10 } else { q.limit };

        let r_result = self.index.search(&giztoy_recall::Query {
            labels: q.labels,
            text: q.text,
            hops,
            limit,
        }).await?;

        let mut entities = Vec::new();
        for label in &r_result.expanded {
            match self.index.graph().get_entity(label) {
                Ok(Some(ent)) => {
                    entities.push(EntityInfo {
                        label: ent.label,
                        attrs: ent.attrs,
                    });
                }
                Ok(None) => continue,
                Err(GraphError::NotFound) => continue,
                Err(e) => return Err(MemoryError::Graph(e)),
            }
        }

        let segments = r_result.segments.into_iter().map(|ss| ScoredSegment {
            id: ss.segment.id,
            summary: ss.segment.summary,
            keywords: ss.segment.keywords,
            labels: ss.segment.labels,
            timestamp: ss.segment.timestamp,
            score: ss.score,
        }).collect();

        Ok(RecallResult { entities, segments })
    }

    /// Store a new segment in this persona's recall index.
    pub async fn store_segment(
        &self,
        input: SegmentInput,
        bucket: giztoy_recall::Bucket,
    ) -> Result<(), MemoryError> {
        let ts = now_nano();
        let seg = Segment {
            id: format!("{}-{}", self.id, ts),
            summary: input.summary,
            keywords: input.keywords,
            labels: input.labels,
            timestamp: ts,
            bucket: bucket.as_str().to_string(),
        };
        self.index.store_segment(&seg).await?;
        Ok(())
    }

    /// Apply entity and relation updates to the graph.
    pub fn apply_entity_update(&self, update: &EntityUpdate) -> Result<(), MemoryError> {
        let g = self.index.graph();

        for e in &update.entities {
            match g.merge_attrs(&e.label, &e.attrs) {
                Ok(()) => {}
                Err(GraphError::NotFound) => {
                    g.set_entity(&Entity {
                        label: e.label.clone(),
                        attrs: e.attrs.clone(),
                    })?;
                }
                Err(e) => return Err(MemoryError::Graph(e)),
            }
        }

        for r in &update.relations {
            g.add_relation(&Relation {
                from: r.from.clone(),
                to: r.to.clone(),
                rel_type: r.rel_type.clone(),
            })?;
        }

        Ok(())
    }

    /// Run the full compression pipeline on a conversation:
    /// 1. Read all messages
    /// 2. ExtractEntities → apply to graph
    /// 3. CompressMessages → store segments in Bucket1H
    /// 4. Clear the conversation
    pub async fn compress(
        &self,
        conv: &Conversation<'_>,
        compressor: Option<&dyn Compressor>,
    ) -> Result<(), MemoryError> {
        let comp: &dyn Compressor = match compressor {
            Some(c) => c,
            None => match &self.compressor {
                Some(c) => c.as_ref(),
                None => return Err(MemoryError::NoCompressor),
            },
        };

        let msgs = conv.all()?;
        if msgs.is_empty() {
            return Ok(());
        }

        let update = comp.extract_entities(&msgs).await?;
        self.apply_entity_update(&update)?;

        let result = comp.compress_messages(&msgs).await?;
        for seg in result.segments {
            self.store_segment(seg, bucket_1h()).await?;
        }

        conv.clear()?;
        Ok(())
    }

    // -- Internal accessors for Conversation and Compact --

    pub(crate) fn kv_store(&self) -> &dyn KVStore {
        self.store.as_ref()
    }

    pub(crate) fn compressor_ref(&self) -> Option<&dyn Compressor> {
        self.compressor.as_deref()
    }
}
