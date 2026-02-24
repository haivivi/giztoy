//! Segmentor trait and types.

use std::collections::HashMap;

use async_trait::async_trait;
use serde::{Deserialize, Deserializer, Serialize};

use crate::error::GenxError;

/// Format a Schema's entity types and attributes into a markdown section.
/// Used by both segmentors and profilers prompt builders.
pub(crate) fn format_schema_section(schema: &Schema, header: &str, intro: &str) -> String {
    let mut sb = String::new();
    sb.push_str(&format!("## {}\n\n", header));
    sb.push_str(intro);
    sb.push_str("\n\n");

    let mut types: Vec<_> = schema.entity_types.iter().collect();
    types.sort_by_key(|(k, _)| (*k).clone());

    for (prefix, es) in types {
        sb.push_str(&format!("### {}\n", prefix));
        if !es.desc.is_empty() {
            sb.push_str(&format!("{}\n", es.desc));
        }
        if !es.attrs.is_empty() {
            sb.push_str("Attributes:\n");
            let mut attrs: Vec<_> = es.attrs.iter().collect();
            attrs.sort_by_key(|(k, _)| (*k).clone());
            for (name, attr) in attrs {
                sb.push_str(&format!("- `{}` ({}): {}\n", name, attr.type_, attr.desc));
            }
        }
        sb.push('\n');
    }
    sb
}

/// Deserialize a value that may be JSON `null` into `Default::default()`.
/// Handles both missing fields (`#[serde(default)]`) and explicit `null`
/// (which Go's `json.Marshal` emits for nil slices/maps).
pub(crate) fn null_default<'de, D, T>(deserializer: D) -> Result<T, D::Error>
where
    D: Deserializer<'de>,
    T: Default + Deserialize<'de>,
{
    Ok(Option::<T>::deserialize(deserializer)?.unwrap_or_default())
}

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
    #[serde(alias = "Segment")]
    pub segment: SegmentOutput,
    #[serde(alias = "Entities", default, deserialize_with = "null_default")]
    pub entities: Vec<EntityOutput>,
    #[serde(alias = "Relations", default, deserialize_with = "null_default")]
    pub relations: Vec<RelationOutput>,
}

/// A compressed conversation fragment.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SegmentOutput {
    pub summary: String,
    #[serde(default, deserialize_with = "null_default")]
    pub keywords: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub labels: Vec<String>,
}

/// An entity extracted from the conversation.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EntityOutput {
    pub label: String,
    #[serde(default, deserialize_with = "null_default", skip_serializing_if = "HashMap::is_empty")]
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
    fn t9_5_null_collections_from_go() {
        let json = r#"{
            "segment": {"summary": "test", "keywords": null, "labels": null},
            "entities": null,
            "relations": null
        }"#;
        let result: SegmentorResult = serde_json::from_str(json).unwrap();
        assert_eq!(result.segment.summary, "test");
        assert!(result.segment.keywords.is_empty());
        assert!(result.segment.labels.is_empty());
        assert!(result.entities.is_empty());
        assert!(result.relations.is_empty());
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
