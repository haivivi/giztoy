use std::collections::{HashMap, HashSet};

use giztoy_kv::{KVError, KVStore};
use serde_json::Value;

use crate::error::GraphError;
use crate::graph::{Entity, Graph, Relation};

/// Default KV key separator.
pub const DEFAULT_SEPARATOR: char = ':';

/// KV key layout (relative to the configured prefix):
///
/// ```text
/// {prefix}{sep}e{sep}{label}                        -> JSON-encoded Entity.Attrs
/// {prefix}{sep}r{sep}{from}{sep}{relType}{sep}{to}  -> empty (forward index)
/// {prefix}{sep}ri{sep}{to}{sep}{relType}{sep}{from} -> empty (reverse index)
/// ```
///
/// The separator defaults to ':' but can be configured (e.g. '\x1F')
/// to allow labels containing ':' such as "person:小明".
pub struct KVGraph {
    store: Box<dyn KVStore>,
    prefix: String,
    sep: char,
}

impl KVGraph {
    /// Create a new KVGraph using the given store, key prefix, and separator.
    ///
    /// The separator is used between key segments. Labels must not contain
    /// the separator character. Use `'\x1F'` (ASCII Unit Separator) to allow
    /// colon-namespaced labels like "person:小明".
    ///
    /// Pass `DEFAULT_SEPARATOR` (`:`) for the default behavior.
    pub fn new(store: Box<dyn KVStore>, prefix: &str) -> Self {
        Self {
            store,
            prefix: prefix.to_string(),
            sep: DEFAULT_SEPARATOR,
        }
    }

    /// Create a KVGraph with a custom separator.
    pub fn with_separator(store: Box<dyn KVStore>, prefix: &str, sep: char) -> Self {
        Self {
            store,
            prefix: prefix.to_string(),
            sep,
        }
    }

    /// Return the separator used by this graph.
    pub fn separator(&self) -> char {
        self.sep
    }

    fn validate_segments(&self, segs: &[&str]) -> Result<(), GraphError> {
        for s in segs {
            if s.contains(self.sep) {
                return Err(GraphError::InvalidLabel(s.to_string()));
            }
        }
        Ok(())
    }

    // --- key helpers ---

    fn entity_key(&self, label: &str) -> String {
        format!("{}{s}e{s}{}", self.prefix, label, s = self.sep)
    }

    fn entity_prefix(&self) -> String {
        format!("{}{s}e{s}", self.prefix, s = self.sep)
    }

    fn fwd_key(&self, from: &str, rel_type: &str, to: &str) -> String {
        format!(
            "{}{s}r{s}{}{s}{}{s}{}",
            self.prefix,
            from,
            rel_type,
            to,
            s = self.sep
        )
    }

    fn fwd_prefix(&self, from: &str) -> String {
        format!("{}{s}r{s}{}{s}", self.prefix, from, s = self.sep)
    }

    fn rev_key(&self, to: &str, rel_type: &str, from: &str) -> String {
        format!(
            "{}{s}ri{s}{}{s}{}{s}{}",
            self.prefix,
            to,
            rel_type,
            from,
            s = self.sep
        )
    }

    fn rev_prefix(&self, to: &str) -> String {
        format!("{}{s}ri{s}{}{s}", self.prefix, to, s = self.sep)
    }

    fn parse_fwd_key(&self, key: &str) -> Option<(String, String, String)> {
        let pfx = format!("{}{s}r{s}", self.prefix, s = self.sep);
        let rest = key.strip_prefix(&pfx)?;
        let mut parts = rest.splitn(3, self.sep);
        let from = parts.next()?.to_string();
        let rel_type = parts.next()?.to_string();
        let to = parts.next()?.to_string();
        if to.contains(self.sep) {
            return None;
        }
        Some((from, rel_type, to))
    }

    fn parse_rev_key(&self, key: &str) -> Option<(String, String, String)> {
        let pfx = format!("{}{s}ri{s}", self.prefix, s = self.sep);
        let rest = key.strip_prefix(&pfx)?;
        let mut parts = rest.splitn(3, self.sep);
        let to = parts.next()?.to_string();
        let rel_type = parts.next()?.to_string();
        let from = parts.next()?.to_string();
        if from.contains(self.sep) {
            return None;
        }
        Some((to, rel_type, from))
    }
}

fn map_kv_err(e: KVError) -> GraphError {
    GraphError::Storage(e.to_string())
}

impl Graph for KVGraph {
    fn get_entity(&self, label: &str) -> Result<Option<Entity>, GraphError> {
        self.validate_segments(&[label])?;
        match self
            .store
            .get(&self.entity_key(label))
            .map_err(map_kv_err)?
        {
            Some(data) => {
                let attrs: HashMap<String, Value> = if data.is_empty() {
                    HashMap::new()
                } else {
                    serde_json::from_slice(&data)
                        .map_err(|e| GraphError::Serialization(e.to_string()))?
                };
                Ok(Some(Entity {
                    label: label.to_string(),
                    attrs,
                }))
            }
            None => Ok(None),
        }
    }

    fn set_entity(&self, entity: &Entity) -> Result<(), GraphError> {
        self.validate_segments(&[&entity.label])?;
        let data = serde_json::to_vec(&entity.attrs)
            .map_err(|e| GraphError::Serialization(e.to_string()))?;
        self.store
            .set(&self.entity_key(&entity.label), &data)
            .map_err(map_kv_err)
    }

    fn delete_entity(&self, label: &str) -> Result<(), GraphError> {
        self.validate_segments(&[label])?;

        // Collect all relation keys involving this entity.
        let rels = self.relations(label)?;

        let mut keys: Vec<String> = Vec::with_capacity(1 + rels.len() * 2);
        keys.push(self.entity_key(label));
        for r in &rels {
            keys.push(self.fwd_key(&r.from, &r.rel_type, &r.to));
            keys.push(self.rev_key(&r.to, &r.rel_type, &r.from));
        }

        let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
        self.store.batch_delete(&key_refs).map_err(map_kv_err)
    }

    fn merge_attrs(&self, label: &str, attrs: &HashMap<String, Value>) -> Result<(), GraphError> {
        self.validate_segments(&[label])?;
        let mut entity = self.get_entity(label)?.ok_or(GraphError::NotFound)?;
        for (k, v) in attrs {
            entity.attrs.insert(k.clone(), v.clone());
        }
        self.set_entity(&entity)
    }

    fn list_entities(&self, prefix: &str) -> Result<Vec<Entity>, GraphError> {
        let kv_prefix = self.entity_prefix();
        let entries = self.store.scan(&kv_prefix).map_err(map_kv_err)?;

        let mut result = Vec::new();
        for (key, value) in entries {
            // Extract label: everything after the entity prefix.
            let label = match key.strip_prefix(&kv_prefix) {
                Some(l) => l,
                None => continue,
            };

            // Filter by prefix if requested.
            if !prefix.is_empty() && !label.starts_with(prefix) {
                continue;
            }

            let attrs: HashMap<String, Value> = if value.is_empty() {
                HashMap::new()
            } else {
                serde_json::from_slice(&value)
                    .map_err(|e| GraphError::Serialization(e.to_string()))?
            };
            result.push(Entity {
                label: label.to_string(),
                attrs,
            });
        }
        Ok(result)
    }

    fn add_relation(&self, relation: &Relation) -> Result<(), GraphError> {
        self.validate_segments(&[&relation.from, &relation.to, &relation.rel_type])?;
        let fwd = self.fwd_key(&relation.from, &relation.rel_type, &relation.to);
        let rev = self.rev_key(&relation.to, &relation.rel_type, &relation.from);
        self.store
            .batch_set(&[(&fwd, &[] as &[u8]), (&rev, &[] as &[u8])])
            .map_err(map_kv_err)
    }

    fn remove_relation(&self, from: &str, to: &str, rel_type: &str) -> Result<(), GraphError> {
        self.validate_segments(&[from, to, rel_type])?;
        let fwd = self.fwd_key(from, rel_type, to);
        let rev = self.rev_key(to, rel_type, from);
        self.store
            .batch_delete(&[fwd.as_str(), rev.as_str()])
            .map_err(map_kv_err)
    }

    fn relations(&self, label: &str) -> Result<Vec<Relation>, GraphError> {
        self.validate_segments(&[label])?;
        let mut rels = Vec::new();

        // Forward: relations where label is the source.
        let fwd_entries = self
            .store
            .scan(&self.fwd_prefix(label))
            .map_err(map_kv_err)?;
        for (key, _) in fwd_entries {
            if let Some((from, rel_type, to)) = self.parse_fwd_key(&key) {
                rels.push(Relation { from, to, rel_type });
            }
        }

        // Reverse: relations where label is the target.
        let rev_entries = self
            .store
            .scan(&self.rev_prefix(label))
            .map_err(map_kv_err)?;
        for (key, _) in rev_entries {
            if let Some((to, rel_type, from)) = self.parse_rev_key(&key) {
                // Skip self-loops: already captured by the forward scan.
                if from == label {
                    continue;
                }
                rels.push(Relation { from, to, rel_type });
            }
        }

        Ok(rels)
    }

    fn neighbors(&self, label: &str, rel_types: &[&str]) -> Result<Vec<String>, GraphError> {
        self.validate_segments(&[label])?;
        for rt in rel_types {
            self.validate_segments(&[rt])?;
        }

        let type_set: HashSet<&str> = rel_types.iter().copied().collect();
        let filter_type = !type_set.is_empty();
        let mut seen = HashSet::new();

        // Outgoing neighbors.
        let fwd_entries = self
            .store
            .scan(&self.fwd_prefix(label))
            .map_err(map_kv_err)?;
        for (key, _) in fwd_entries {
            if let Some((_from, rel_type, to)) = self.parse_fwd_key(&key) {
                if filter_type && !type_set.contains(rel_type.as_str()) {
                    continue;
                }
                seen.insert(to);
            }
        }

        // Incoming neighbors.
        let rev_entries = self
            .store
            .scan(&self.rev_prefix(label))
            .map_err(map_kv_err)?;
        for (key, _) in rev_entries {
            if let Some((_to, rel_type, from)) = self.parse_rev_key(&key) {
                if filter_type && !type_set.contains(rel_type.as_str()) {
                    continue;
                }
                seen.insert(from);
            }
        }

        let mut result: Vec<String> = seen.into_iter().collect();
        result.sort();
        Ok(result)
    }

    fn expand(&self, seeds: &[&str], hops: usize) -> Result<Vec<String>, GraphError> {
        for s in seeds {
            self.validate_segments(&[s])?;
        }

        let mut visited: HashSet<String> = seeds.iter().map(|s| s.to_string()).collect();
        let mut frontier: Vec<String> = seeds.iter().map(|s| s.to_string()).collect();

        for _ in 0..hops {
            if frontier.is_empty() {
                break;
            }
            let mut next = Vec::new();
            for label in &frontier {
                let neighbors = self.neighbors(label, &[])?;
                for n in neighbors {
                    if !visited.contains(&n) {
                        visited.insert(n.clone());
                        next.push(n);
                    }
                }
            }
            frontier = next;
        }

        let mut result: Vec<String> = visited.into_iter().collect();
        result.sort();
        Ok(result)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use giztoy_kv::RedbStore;

    fn new_test_graph() -> KVGraph {
        let dir = tempfile::tempdir().unwrap();
        let db_path = dir.path().join("test.redb");
        let store = RedbStore::open(&db_path).unwrap();
        // Leak the tempdir so it stays alive for the test.
        // (Tests are short-lived, this is fine.)
        std::mem::forget(dir);
        KVGraph::new(Box::new(store), "test:g")
    }

    // --- Validation tests ---

    #[test]
    fn test_invalid_label_separator() {
        let g = new_test_graph();

        // Entity operations with label containing separator.
        assert!(matches!(
            g.get_entity("person:123"),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.set_entity(&Entity {
                label: "person:123".into(),
                attrs: HashMap::new(),
            }),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.delete_entity("a:b"),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.merge_attrs("a:b", &HashMap::new()),
            Err(GraphError::InvalidLabel(_))
        ));

        // Relation operations.
        assert!(matches!(
            g.add_relation(&Relation {
                from: "a:b".into(),
                to: "c".into(),
                rel_type: "knows".into(),
            }),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.add_relation(&Relation {
                from: "a".into(),
                to: "b:c".into(),
                rel_type: "knows".into(),
            }),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.add_relation(&Relation {
                from: "a".into(),
                to: "b".into(),
                rel_type: "type:sub".into(),
            }),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.remove_relation("a:b", "c", "knows"),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.relations("a:b"),
            Err(GraphError::InvalidLabel(_))
        ));

        // Traversal operations.
        assert!(matches!(
            g.neighbors("a:b", &[]),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.neighbors("a", &["type:sub"]),
            Err(GraphError::InvalidLabel(_))
        ));
        assert!(matches!(
            g.expand(&["ok", "bad:label"], 1),
            Err(GraphError::InvalidLabel(_))
        ));
    }

    // --- Entity tests ---

    #[test]
    fn test_get_entity_not_found() {
        let g = new_test_graph();
        assert!(g.get_entity("nobody").unwrap().is_none());
    }

    #[test]
    fn test_set_get_entity() {
        let g = new_test_graph();
        let e = Entity {
            label: "Alice".into(),
            attrs: HashMap::from([
                ("age".into(), serde_json::json!(30.0)),
                ("role".into(), serde_json::json!("engineer")),
            ]),
        };
        g.set_entity(&e).unwrap();

        let got = g.get_entity("Alice").unwrap().unwrap();
        assert_eq!(got.label, "Alice");
        assert_eq!(got.attrs["role"], serde_json::json!("engineer"));
        assert_eq!(got.attrs["age"], serde_json::json!(30.0));
    }

    #[test]
    fn test_set_entity_overwrite() {
        let g = new_test_graph();
        g.set_entity(&Entity {
            label: "Bob".into(),
            attrs: HashMap::from([("v".into(), serde_json::json!(1))]),
        })
        .unwrap();
        g.set_entity(&Entity {
            label: "Bob".into(),
            attrs: HashMap::from([("v".into(), serde_json::json!(2))]),
        })
        .unwrap();

        let got = g.get_entity("Bob").unwrap().unwrap();
        assert_eq!(got.attrs["v"], serde_json::json!(2));
    }

    #[test]
    fn test_set_entity_no_attrs() {
        let g = new_test_graph();
        g.set_entity(&Entity {
            label: "Empty".into(),
            attrs: HashMap::new(),
        })
        .unwrap();
        let got = g.get_entity("Empty").unwrap().unwrap();
        assert_eq!(got.label, "Empty");
    }

    #[test]
    fn test_delete_entity() {
        let g = new_test_graph();
        g.set_entity(&Entity {
            label: "A".into(),
            attrs: HashMap::new(),
        })
        .unwrap();
        g.set_entity(&Entity {
            label: "B".into(),
            attrs: HashMap::new(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        })
        .unwrap();

        g.delete_entity("A").unwrap();

        assert!(g.get_entity("A").unwrap().is_none());
        let rels = g.relations("B").unwrap();
        assert_eq!(rels.len(), 0);
    }

    #[test]
    fn test_delete_entity_non_existent() {
        let g = new_test_graph();
        g.delete_entity("ghost").unwrap();
    }

    #[test]
    fn test_merge_attrs() {
        let g = new_test_graph();
        g.set_entity(&Entity {
            label: "X".into(),
            attrs: HashMap::from([
                ("a".into(), serde_json::json!("1")),
                ("b".into(), serde_json::json!("2")),
            ]),
        })
        .unwrap();

        g.merge_attrs(
            "X",
            &HashMap::from([
                ("b".into(), serde_json::json!("updated")),
                ("c".into(), serde_json::json!("3")),
            ]),
        )
        .unwrap();

        let got = g.get_entity("X").unwrap().unwrap();
        assert_eq!(got.attrs["a"], serde_json::json!("1"));
        assert_eq!(got.attrs["b"], serde_json::json!("updated"));
        assert_eq!(got.attrs["c"], serde_json::json!("3"));
    }

    #[test]
    fn test_merge_attrs_not_found() {
        let g = new_test_graph();
        let result = g.merge_attrs(
            "ghost",
            &HashMap::from([("a".into(), serde_json::json!("1"))]),
        );
        assert!(matches!(result, Err(GraphError::NotFound)));
    }

    #[test]
    fn test_list_entities() {
        let g = new_test_graph();
        for label in &["Alice", "Alex", "Bob", "Charlie"] {
            g.set_entity(&Entity {
                label: label.to_string(),
                attrs: HashMap::new(),
            })
            .unwrap();
        }

        // List all.
        let all: Vec<String> = g
            .list_entities("")
            .unwrap()
            .into_iter()
            .map(|e| e.label)
            .collect();
        assert_eq!(all, vec!["Alex", "Alice", "Bob", "Charlie"]);

        // List with prefix "Al".
        let filtered: Vec<String> = g
            .list_entities("Al")
            .unwrap()
            .into_iter()
            .map(|e| e.label)
            .collect();
        assert_eq!(filtered, vec!["Alex", "Alice"]);
    }

    // --- Relation tests ---

    #[test]
    fn test_add_and_get_relations() {
        let g = new_test_graph();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "C".into(),
            rel_type: "works_with".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "D".into(),
            to: "A".into(),
            rel_type: "manages".into(),
        })
        .unwrap();

        let rels = g.relations("A").unwrap();
        assert_eq!(rels.len(), 3);

        let rels = g.relations("B").unwrap();
        assert_eq!(rels.len(), 1);
        assert_eq!(rels[0].from, "A");
        assert_eq!(rels[0].to, "B");
        assert_eq!(rels[0].rel_type, "knows");
    }

    #[test]
    fn test_add_relation_idempotent() {
        let g = new_test_graph();
        let r = Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        };
        g.add_relation(&r).unwrap();
        g.add_relation(&r).unwrap();

        let rels = g.relations("A").unwrap();
        assert_eq!(rels.len(), 1);
    }

    #[test]
    fn test_relations_self_loop() {
        let g = new_test_graph();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "A".into(),
            rel_type: "self".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        })
        .unwrap();

        let rels = g.relations("A").unwrap();
        assert_eq!(rels.len(), 2);

        let neighbors = g.neighbors("A", &[]).unwrap();
        assert_eq!(neighbors, vec!["A", "B"]);
    }

    #[test]
    fn test_remove_relation() {
        let g = new_test_graph();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "C".into(),
            rel_type: "knows".into(),
        })
        .unwrap();

        g.remove_relation("A", "B", "knows").unwrap();

        let rels = g.relations("A").unwrap();
        assert_eq!(rels.len(), 1);
        assert_eq!(rels[0].to, "C");

        let rels = g.relations("B").unwrap();
        assert_eq!(rels.len(), 0);
    }

    #[test]
    fn test_remove_relation_non_existent() {
        let g = new_test_graph();
        g.remove_relation("X", "Y", "nope").unwrap();
    }

    // --- Traversal tests ---

    #[test]
    fn test_neighbors() {
        let g = new_test_graph();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "C".into(),
            rel_type: "works_with".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "D".into(),
            to: "A".into(),
            rel_type: "manages".into(),
        })
        .unwrap();

        // All neighbors of A.
        let got = g.neighbors("A", &[]).unwrap();
        assert_eq!(got, vec!["B", "C", "D"]);

        // Filtered by "knows".
        let got = g.neighbors("A", &["knows"]).unwrap();
        assert_eq!(got, vec!["B"]);

        // Neighbors of B (incoming from A).
        let got = g.neighbors("B", &[]).unwrap();
        assert_eq!(got, vec!["A"]);
    }

    #[test]
    fn test_neighbors_multiple_rel_types() {
        let g = new_test_graph();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "knows".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "C".into(),
            rel_type: "works_with".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "A".into(),
            to: "D".into(),
            rel_type: "manages".into(),
        })
        .unwrap();

        let got = g.neighbors("A", &["knows", "manages"]).unwrap();
        assert_eq!(got, vec!["B", "D"]);
    }

    #[test]
    fn test_expand_zero_hops() {
        let g = new_test_graph();
        let got = g.expand(&["A", "B"], 0).unwrap();
        assert_eq!(got, vec!["A", "B"]);
    }

    #[test]
    fn test_expand_multi_hop() {
        let g = new_test_graph();
        // Chain: A -> B -> C -> D -> E
        for (from, to) in [("A", "B"), ("B", "C"), ("C", "D"), ("D", "E")] {
            g.add_relation(&Relation {
                from: from.into(),
                to: to.into(),
                rel_type: "next".into(),
            })
            .unwrap();
        }

        // 1 hop from A: A + B
        let got = g.expand(&["A"], 1).unwrap();
        assert_eq!(got, vec!["A", "B"]);

        // 2 hops from A: A + B + C (B's neighbors include A via reverse, already visited)
        let got = g.expand(&["A"], 2).unwrap();
        assert_eq!(got, vec!["A", "B", "C"]);

        // 3 hops from A: A + B + C + D
        let got = g.expand(&["A"], 3).unwrap();
        assert_eq!(got, vec!["A", "B", "C", "D"]);
    }

    #[test]
    fn test_expand_graph() {
        let g = new_test_graph();
        //    A
        //   / \
        //  B   C
        //   \ /
        //    D
        for (from, to) in [("A", "B"), ("A", "C"), ("B", "D"), ("C", "D")] {
            g.add_relation(&Relation {
                from: from.into(),
                to: to.into(),
                rel_type: "link".into(),
            })
            .unwrap();
        }

        let got = g.expand(&["A"], 2).unwrap();
        assert_eq!(got, vec!["A", "B", "C", "D"]);
    }

    #[test]
    fn test_expand_multiple_seeds() {
        let g = new_test_graph();
        // Two disconnected pairs: A-B, C-D
        g.add_relation(&Relation {
            from: "A".into(),
            to: "B".into(),
            rel_type: "link".into(),
        })
        .unwrap();
        g.add_relation(&Relation {
            from: "C".into(),
            to: "D".into(),
            rel_type: "link".into(),
        })
        .unwrap();

        let got = g.expand(&["A", "C"], 1).unwrap();
        assert_eq!(got, vec!["A", "B", "C", "D"]);
    }
}
