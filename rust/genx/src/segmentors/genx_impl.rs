//! GenX segmentor implementation using a Generator (LLM).

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use serde::Deserialize;

use super::prompt::{build_conversation_text, build_prompt};
use super::types::*;
use crate::context::ModelContextBuilder;
use crate::error::GenxError;
use crate::generators::Mux as GeneratorMux;
use crate::tool::FuncTool;
use crate::Generator;

/// Typed argument for the FuncTool, matching the expected JSON output from the LLM.
///
/// `attrs` uses Vec<ExtractAttr> instead of HashMap because OpenAI strict mode
/// requires additionalProperties:false on all objects.
#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ExtractArg {
    segment: ExtractSegment,
    entities: Vec<ExtractEntity>,
    relations: Vec<ExtractRelation>,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ExtractSegment {
    summary: String,
    keywords: Vec<String>,
    labels: Vec<String>,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ExtractEntity {
    label: String,
    attrs: Vec<ExtractAttr>,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ExtractAttr {
    key: String,
    value: String,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ExtractRelation {
    from: String,
    to: String,
    rel_type: String,
}

/// GenX segmentor — calls an LLM Generator to extract segments.
pub struct GenXSegmentor {
    generator_pattern: String,
    mux: Option<Arc<GeneratorMux>>,
}

impl GenXSegmentor {
    pub fn new(cfg: SegmentorConfig) -> Self {
        Self {
            generator_pattern: cfg.generator,
            mux: None,
        }
    }

    pub fn with_mux(cfg: SegmentorConfig, mux: Arc<GeneratorMux>) -> Self {
        Self {
            generator_pattern: cfg.generator,
            mux: Some(mux),
        }
    }

    fn get_generator(&self) -> Result<&dyn Generator, GenxError> {
        match &self.mux {
            Some(mux) => Ok(mux.as_ref()),
            None => Err(GenxError::Other(anyhow::anyhow!(
                "segmentors: no generator mux configured"
            ))),
        }
    }

    fn parse_result(&self, arguments: &str) -> Result<SegmentorResult, GenxError> {
        let arg: ExtractArg = serde_json::from_str(arguments).map_err(|e| {
            GenxError::Other(anyhow::anyhow!(
                "segmentors: failed to parse extraction result: {}",
                e
            ))
        })?;

        let entities = arg
            .entities
            .into_iter()
            .map(|e| {
                let attrs: HashMap<String, serde_json::Value> = e
                    .attrs
                    .into_iter()
                    .map(|a| (a.key, serde_json::Value::String(a.value)))
                    .collect();
                EntityOutput {
                    label: e.label,
                    attrs,
                }
            })
            .collect();

        let relations = arg
            .relations
            .into_iter()
            .map(|r| RelationOutput {
                from: r.from,
                to: r.to,
                rel_type: r.rel_type,
            })
            .collect();

        Ok(SegmentorResult {
            segment: SegmentOutput {
                summary: arg.segment.summary,
                keywords: arg.segment.keywords,
                labels: arg.segment.labels,
            },
            entities,
            relations,
        })
    }
}

#[async_trait]
impl Segmentor for GenXSegmentor {
    async fn process(&self, input: SegmentorInput) -> Result<SegmentorResult, GenxError> {
        let mut mcb = ModelContextBuilder::new();
        mcb.prompt_text("segmentor", build_prompt(&input));
        mcb.user_text("conversation", build_conversation_text(&input.messages));
        let mctx = mcb.build();

        let extract_tool = FuncTool::new::<ExtractArg>(
            "extract",
            "Extract a compressed segment with entities and relations from the conversation.",
        );

        let generator = self.get_generator()?;
        let (_usage, call) = generator
            .invoke(&self.generator_pattern, &mctx, &extract_tool)
            .await?;

        self.parse_result(&call.arguments)
    }

    fn model(&self) -> &str {
        &self.generator_pattern
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn t10_5_parse_result_valid() {
        let seg = GenXSegmentor::new(SegmentorConfig {
            generator: "test".into(),
            prompt_version: None,
        });

        let json = r#"{
            "segment": {
                "summary": "小明聊了恐龙",
                "keywords": ["恐龙"],
                "labels": ["person:小明", "topic:恐龙"]
            },
            "entities": [
                {
                    "label": "person:小明",
                    "attrs": [{"key": "age", "value": "5"}]
                }
            ],
            "relations": [
                {"from": "person:小明", "to": "topic:恐龙", "rel_type": "likes"}
            ]
        }"#;

        let result = seg.parse_result(json).unwrap();
        assert_eq!(result.segment.summary, "小明聊了恐龙");
        assert_eq!(result.entities.len(), 1);
        assert_eq!(
            result.entities[0].attrs.get("age"),
            Some(&serde_json::Value::String("5".into()))
        );
        assert_eq!(result.relations.len(), 1);
        assert_eq!(result.relations[0].rel_type, "likes");
    }

    #[test]
    fn t10_6_parse_result_invalid() {
        let seg = GenXSegmentor::new(SegmentorConfig {
            generator: "test".into(),
            prompt_version: None,
        });
        let result = seg.parse_result("not valid json");
        assert!(result.is_err());
    }

    #[test]
    fn t10_7_parse_result_empty_entities() {
        let seg = GenXSegmentor::new(SegmentorConfig {
            generator: "test".into(),
            prompt_version: None,
        });

        let json = r#"{
            "segment": {"summary": "test", "keywords": [], "labels": []},
            "entities": [],
            "relations": []
        }"#;

        let result = seg.parse_result(json).unwrap();
        assert!(result.entities.is_empty());
        assert!(result.relations.is_empty());
    }
}
