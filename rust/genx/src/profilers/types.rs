//! Profiler trait and types.

use std::collections::HashMap;

use async_trait::async_trait;
use serde::{Deserialize, Serialize};

use crate::error::GenxError;
use crate::segmentors::{AttrDef, RelationOutput, Schema, SegmentorResult};

/// Evolves entity profile schemas and updates profiles.
#[async_trait]
pub trait Profiler: Send + Sync {
    async fn process(&self, input: ProfilerInput) -> Result<ProfilerResult, GenxError>;
    fn model(&self) -> &str;
}

/// Input to a Profiler.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProfilerInput {
    pub messages: Vec<String>,
    pub extracted: SegmentorResult,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub schema: Option<Schema>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profiles: Option<HashMap<String, HashMap<String, serde_json::Value>>>,
}

/// Output of a Profiler.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ProfilerResult {
    #[serde(default)]
    pub schema_changes: Vec<SchemaChange>,
    #[serde(default)]
    pub profile_updates: HashMap<String, HashMap<String, serde_json::Value>>,
    #[serde(default)]
    pub relations: Vec<RelationOutput>,
}

/// A proposed modification to the entity type schema.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SchemaChange {
    pub entity_type: String,
    pub field: String,
    pub def: AttrDef,
    pub action: String,
}

/// Config for a GenX profiler.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProfilerConfig {
    pub generator: String,
    #[serde(default)]
    pub prompt_version: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn t12_1_result_json_roundtrip() {
        let result = ProfilerResult {
            schema_changes: vec![SchemaChange {
                entity_type: "person".into(),
                field: "school".into(),
                def: AttrDef {
                    type_: "string".into(),
                    desc: "学校名称".into(),
                },
                action: "add".into(),
            }],
            profile_updates: {
                let mut outer = HashMap::new();
                let mut inner = HashMap::new();
                inner.insert("age".into(), serde_json::json!(5));
                outer.insert("person:小明".into(), inner);
                outer
            },
            relations: vec![RelationOutput {
                from: "person:小明".into(),
                to: "place:阳光幼儿园".into(),
                rel_type: "attends".into(),
            }],
        };
        let json = serde_json::to_string(&result).unwrap();
        let parsed: ProfilerResult = serde_json::from_str(&json).unwrap();
        assert_eq!(result, parsed);
    }

    #[test]
    fn t12_2_schema_change_actions() {
        let add = SchemaChange {
            entity_type: "person".into(),
            field: "school".into(),
            def: AttrDef {
                type_: "string".into(),
                desc: "School".into(),
            },
            action: "add".into(),
        };
        let modify = SchemaChange {
            action: "modify".into(),
            ..add.clone()
        };
        assert_eq!(add.action, "add");
        assert_eq!(modify.action, "modify");
    }
}
