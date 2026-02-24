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
    generator: Option<Arc<dyn Generator>>,
}

impl GenXSegmentor {
    pub fn new(cfg: SegmentorConfig) -> Self {
        Self {
            generator_pattern: cfg.generator,
            generator: None,
        }
    }

    pub fn with_mux(cfg: SegmentorConfig, mux: Arc<GeneratorMux>) -> Self {
        Self {
            generator_pattern: cfg.generator,
            generator: Some(mux),
        }
    }

    pub fn with_generator(cfg: SegmentorConfig, generator: Arc<dyn Generator>) -> Self {
        Self {
            generator_pattern: cfg.generator,
            generator: Some(generator),
        }
    }

    fn get_generator(&self) -> Result<&dyn Generator, GenxError> {
        match &self.generator {
            Some(g) => Ok(g.as_ref()),
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

    #[test]
    fn t_segmentor_model() {
        let seg = GenXSegmentor::new(SegmentorConfig {
            generator: "qwen/turbo".into(),
            prompt_version: None,
        });
        assert_eq!(seg.model(), "qwen/turbo");
    }

    #[tokio::test]
    async fn t_segmentor_invoke_error() {
        let seg = GenXSegmentor::new(SegmentorConfig {
            generator: "missing".into(),
            prompt_version: None,
        });
        let input = SegmentorInput {
            messages: vec!["user: test".into()],
            schema: None,
        };
        let err = seg.process(input).await.unwrap_err();
        assert!(err.to_string().contains("no generator mux"));
    }

    #[test]
    fn t_segmentor_parse_nil_call() {
        let seg = GenXSegmentor::new(SegmentorConfig {
            generator: "test".into(),
            prompt_version: None,
        });
        let result = seg.parse_result("");
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn t_segmentor_process_mock_generator() {
        use crate::generators::Mux as GeneratorMux;
        use crate::context::ModelContext;
        use crate::stream::Stream;

        struct MockGen { response: String }

        #[async_trait::async_trait]
        impl crate::Generator for MockGen {
            async fn generate_stream(&self, _: &str, _: &dyn ModelContext)
                -> Result<Box<dyn Stream>, crate::GenxError> { unimplemented!() }
            async fn invoke(&self, _: &str, _: &dyn ModelContext, tool: &crate::FuncTool)
                -> Result<(crate::Usage, crate::types::FuncCall), crate::GenxError> {
                use crate::tool::Tool;
                Ok((crate::Usage::default(), crate::types::FuncCall::new(tool.name(), &self.response)))
            }
        }

        let mock_json = r#"{
            "segment": {
                "summary": "小明和爸爸聊了恐龙",
                "keywords": ["恐龙", "霸王龙", "小明"],
                "labels": ["person:小明", "person:爸爸", "topic:恐龙"]
            },
            "entities": [
                {"label": "person:小明", "attrs": [{"key": "age", "value": "5"}, {"key": "favorite_dinosaur", "value": "霸王龙"}]},
                {"label": "person:爸爸", "attrs": []},
                {"label": "topic:恐龙", "attrs": [{"key": "category", "value": "古生物"}]}
            ],
            "relations": [
                {"from": "person:小明", "to": "topic:恐龙", "rel_type": "likes"},
                {"from": "person:爸爸", "to": "person:小明", "rel_type": "parent"}
            ]
        }"#;

        let mut gen_mux = GeneratorMux::new();
        gen_mux.handle("mock/gen", std::sync::Arc::new(MockGen { response: mock_json.into() })).unwrap();

        let seg = GenXSegmentor::with_mux(
            SegmentorConfig { generator: "mock/gen".into(), prompt_version: None },
            std::sync::Arc::new(gen_mux),
        );

        let input = SegmentorInput {
            messages: vec!["user: 今天和小明聊了恐龙".into(), "assistant: 小明最喜欢霸王龙".into()],
            schema: None,
        };

        let result = seg.process(input).await.unwrap();

        assert!(!result.segment.summary.is_empty());
        assert_eq!(result.segment.keywords.len(), 3);
        assert_eq!(result.segment.labels.len(), 3);
        assert_eq!(result.entities.len(), 3);
        let xiaoming = result.entities.iter().find(|e| e.label == "person:小明").unwrap();
        assert_eq!(xiaoming.attrs.get("favorite_dinosaur"), Some(&serde_json::Value::String("霸王龙".into())));
        assert_eq!(result.relations.len(), 2);
        assert!(result.relations.iter().any(|r| r.from == "person:小明" && r.to == "topic:恐龙" && r.rel_type == "likes"));
    }
}
