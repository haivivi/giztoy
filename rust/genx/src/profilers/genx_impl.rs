//! GenX profiler implementation using a Generator (LLM).

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;
use serde::Deserialize;

use super::prompt::{build_conversation_text, build_prompt};
use super::types::*;
use crate::context::ModelContextBuilder;
use crate::error::GenxError;
use crate::generators::Mux as GeneratorMux;
use crate::segmentors::RelationOutput;
use crate::tool::FuncTool;
use crate::Generator;

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ProfileArg {
    schema_changes: Vec<ProfileSchemaChange>,
    profile_updates: HashMap<String, HashMap<String, serde_json::Value>>,
    relations: Vec<ProfileRelation>,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ProfileSchemaChange {
    entity_type: String,
    field: String,
    def: ProfileAttrDef,
    action: String,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ProfileAttrDef {
    #[serde(rename = "type")]
    type_: String,
    desc: String,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
struct ProfileRelation {
    from: String,
    to: String,
    rel_type: String,
}

/// GenX profiler — calls an LLM Generator for profile analysis.
pub struct GenXProfiler {
    generator_pattern: String,
    mux: Option<Arc<GeneratorMux>>,
}

impl GenXProfiler {
    pub fn new(cfg: ProfilerConfig) -> Self {
        Self {
            generator_pattern: cfg.generator,
            mux: None,
        }
    }

    pub fn with_mux(cfg: ProfilerConfig, mux: Arc<GeneratorMux>) -> Self {
        Self {
            generator_pattern: cfg.generator,
            mux: Some(mux),
        }
    }

    fn get_generator(&self) -> Result<&dyn Generator, GenxError> {
        match &self.mux {
            Some(mux) => Ok(mux.as_ref()),
            None => Err(GenxError::Other(anyhow::anyhow!(
                "profilers: no generator mux configured"
            ))),
        }
    }

    fn parse_result(&self, arguments: &str) -> Result<ProfilerResult, GenxError> {
        let arg: ProfileArg = serde_json::from_str(arguments).map_err(|e| {
            GenxError::Other(anyhow::anyhow!(
                "profilers: failed to parse profile result: {}",
                e
            ))
        })?;

        Ok(ProfilerResult {
            schema_changes: arg
                .schema_changes
                .into_iter()
                .map(|sc| SchemaChange {
                    entity_type: sc.entity_type,
                    field: sc.field,
                    def: crate::segmentors::AttrDef {
                        type_: sc.def.type_,
                        desc: sc.def.desc,
                    },
                    action: sc.action,
                })
                .collect(),
            profile_updates: arg.profile_updates,
            relations: arg
                .relations
                .into_iter()
                .map(|r| RelationOutput {
                    from: r.from,
                    to: r.to,
                    rel_type: r.rel_type,
                })
                .collect(),
        })
    }
}

#[async_trait]
impl Profiler for GenXProfiler {
    async fn process(&self, input: ProfilerInput) -> Result<ProfilerResult, GenxError> {
        let mut mcb = ModelContextBuilder::new();
        mcb.prompt_text("profiler", build_prompt(&input));
        mcb.user_text("conversation", build_conversation_text(&input.messages));
        let mctx = mcb.build();

        let profile_tool = FuncTool::new::<ProfileArg>(
            "update_profiles",
            "Update entity profiles and propose schema changes based on conversation analysis.",
        );

        let generator = self.get_generator()?;
        let (_usage, call) = generator
            .invoke(&self.generator_pattern, &mctx, &profile_tool)
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
    fn t12_4_parse_result_valid() {
        let prof = GenXProfiler::new(ProfilerConfig {
            generator: "test".into(),
            prompt_version: None,
        });

        let json = r#"{
            "schema_changes": [{
                "entity_type": "person",
                "field": "school",
                "def": {"type": "string", "desc": "School name"},
                "action": "add"
            }],
            "profile_updates": {
                "person:小明": {"age": 5}
            },
            "relations": [{
                "from": "person:小明",
                "to": "place:school",
                "rel_type": "attends"
            }]
        }"#;

        let result = prof.parse_result(json).unwrap();
        assert_eq!(result.schema_changes.len(), 1);
        assert_eq!(result.schema_changes[0].action, "add");
        assert!(result.profile_updates.contains_key("person:小明"));
        assert_eq!(result.relations.len(), 1);
    }

    #[test]
    fn t12_profilers_uses_segmentors_types() {
        use crate::segmentors::{SegmentOutput, SegmentorResult};
        let _result = SegmentorResult {
            segment: SegmentOutput {
                summary: "test".into(),
                keywords: vec![],
                labels: vec![],
            },
            entities: vec![],
            relations: vec![],
        };
    }
}
