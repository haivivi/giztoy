use std::collections::HashMap;

use serde_json::Value;

use crate::error::GraphError;

/// Entity is a node in the graph identified by a unique label.
#[derive(Debug, Clone)]
pub struct Entity {
    /// Unique identifier for this entity.
    pub label: String,

    /// Arbitrary key-value attributes associated with the entity.
    /// Values are JSON-compatible (matching Go's `map[string]any`).
    pub attrs: HashMap<String, Value>,
}

/// Relation is a typed directed edge between two entities.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Relation {
    /// Source entity label.
    pub from: String,

    /// Target entity label.
    pub to: String,

    /// Relation type (e.g., "knows", "works_at").
    pub rel_type: String,
}

/// Graph is the interface for an entity-relation graph.
///
/// All implementations must be safe for concurrent use (Send + Sync).
pub trait Graph: Send + Sync {
    // --- Entity operations ---

    /// Retrieve an entity by label. Returns `None` if not present.
    fn get_entity(&self, label: &str) -> Result<Option<Entity>, GraphError>;

    /// Create or overwrite an entity.
    fn set_entity(&self, entity: &Entity) -> Result<(), GraphError>;

    /// Remove an entity and all its relations (both directions).
    fn delete_entity(&self, label: &str) -> Result<(), GraphError>;

    /// Merge the given attributes into an existing entity.
    /// New keys are added; existing keys are overwritten. Keys not in `attrs`
    /// are left unchanged. Returns `GraphError::NotFound` if the entity does
    /// not exist.
    fn merge_attrs(
        &self,
        label: &str,
        attrs: &HashMap<String, Value>,
    ) -> Result<(), GraphError>;

    /// List entities whose label starts with `prefix`. Pass `""` for all.
    fn list_entities(&self, prefix: &str) -> Result<Vec<Entity>, GraphError>;

    // --- Relation operations ---

    /// Create a directed relation. Idempotent: no-op if already exists.
    fn add_relation(&self, relation: &Relation) -> Result<(), GraphError>;

    /// Remove a specific relation. No error if it does not exist.
    fn remove_relation(
        &self,
        from: &str,
        to: &str,
        rel_type: &str,
    ) -> Result<(), GraphError>;

    /// Return all relations where the given label is source or target.
    fn relations(&self, label: &str) -> Result<Vec<Relation>, GraphError>;

    // --- Traversal ---

    /// Return labels of entities directly connected to the given label.
    /// If `rel_types` is non-empty, only relations of those types are
    /// considered. Returns labels from both directions.
    fn neighbors(
        &self,
        label: &str,
        rel_types: &[&str],
    ) -> Result<Vec<String>, GraphError>;

    /// Multi-hop BFS expansion from seed labels, returning all discovered
    /// labels (including seeds). `hops` controls maximum traversal depth
    /// (0 returns only the seeds).
    fn expand(
        &self,
        seeds: &[&str],
        hops: usize,
    ) -> Result<Vec<String>, GraphError>;
}
