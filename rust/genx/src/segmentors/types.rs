//! Segmentor trait and types.

use std::collections::HashMap;

use async_trait::async_trait;
use serde::{Deserialize, Serialize};

use crate::error::GenxError;

/// Compresses conversation messages into a structured segment
/// with entity and relation extraction.
#[async_trait]
pub trait Segmentor: Send + Sync {
    async fn process(&self, input: SegmentorInput) -> Result<SegmentorResult, GenxError>;
    fn model(&self) -> &str;
}

/// Input to a Segmentor.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SegmentorInput {
    pub messages: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub schema: Option<Schema>,
}

/// Output of a Segmentor.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SegmentorResult {
    pub segment: SegmentOutput,
    pub entities: Vec<EntityOutput>,
    pub relations: Vec<RelationOutput>,
}

/// A compressed conversation fragment.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SegmentOutput {
    pub summary: String,
    #[serde(default)]
    pub keywords: Vec<String>,
    #[serde(default)]
    pub labels: Vec<String>,
}

/// An entity extracted from the conversation.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EntityOutput {
    pub label: String,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub attrs: HashMap<String, serde_json::Value>,
}

/// A directed relation between two entities.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RelationOutput {
    pub from: String,
    pub to: String,
    pub rel_type: String,
}

/// Schema describing entity types and expected attributes.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Schema {
    #[serde(default)]
    pub entity_types: HashMap<String, EntitySchema>,
}

/// Schema for a single entity type.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EntitySchema {
    #[serde(default)]
    pub desc: String,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub attrs: HashMap<String, AttrDef>,
}

/// Definition of an entity attribute.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct AttrDef {
    #[serde(rename = "type")]
    pub type_: String,
    pub desc: String,
}

/// Config for a GenX segmentor.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SegmentorConfig {
    pub generator: String,
    #[serde(default)]
    pub prompt_version: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn t9_1_result_json_roundtrip() {
        let result = SegmentorResult {
            segment: SegmentOutput {
                summary: "小明和小红聊了恐龙".into(),
                keywords: vec!["恐龙".into(), "小明".into()],
                labels: vec!["person:小明".into(), "topic:恐龙".into()],
            },
            entities: vec![EntityOutput {
                label: "person:小明".into(),
                attrs: {
                    let mut m = HashMap::new();
                    m.insert("age".into(), serde_json::json!("5"));
                    m
                },
            }],
            relations: vec![RelationOutput {
                from: "person:小明".into(),
                to: "topic:恐龙".into(),
                rel_type: "likes".into(),
            }],
        };
        let json = serde_json::to_string(&result).unwrap();
        let parsed: SegmentorResult = serde_json::from_str(&json).unwrap();
        assert_eq!(result, parsed);
    }

    #[test]
    fn t9_2_schema_yaml() {
        let yaml = r#"
entity_types:
  person:
    desc: "A person"
    attrs:
      age:
        type: string
        desc: "Age"
      name:
        type: string
        desc: "Name"
  topic:
    desc: "A topic"
"#;
        let schema: Schema = serde_yaml::from_str(yaml).unwrap();
        assert!(schema.entity_types.contains_key("person"));
        assert!(schema.entity_types.contains_key("topic"));
        let person = &schema.entity_types["person"];
        assert_eq!(person.attrs.len(), 2);
    }

    #[test]
    fn t9_3_empty_attrs_omitted() {
        let entity = EntityOutput {
            label: "topic:恐龙".into(),
            attrs: HashMap::new(),
        };
        let json = serde_json::to_string(&entity).unwrap();
        assert!(!json.contains("attrs"));
    }

    #[test]
    fn t9_4_relation_output_serialization() {
        let rel = RelationOutput {
            from: "person:小明".into(),
            to: "topic:恐龙".into(),
            rel_type: "likes".into(),
        };
        let json = serde_json::to_string(&rel).unwrap();
        assert!(json.contains("person:小明"));
        assert!(json.contains("likes"));
        let parsed: RelationOutput = serde_json::from_str(&json).unwrap();
        assert_eq!(rel, parsed);
    }
}
