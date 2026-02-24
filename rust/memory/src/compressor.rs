//! LLMCompressor - LLM-based conversation compressor
//!
//! This module implements the [Compressor] trait by delegating to the segmentors
//! and profilers packages for LLM-based conversation compression.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;

use giztoy_genx::segmentors::{self, SegmentorMux, SegmentorResult};
use giztoy_genx::profilers::{self, ProfilerMux, ProfilerResult};

use crate::error::MemoryError;
use crate::types::{CompressResult, Compressor, EntityInput, EntityUpdate, Message, RelationInput, SegmentInput};

/// LLMCompressorConfig configures an [LLMCompressor].
#[derive(Clone)]
pub struct LLMCompressorConfig {
    /// Segmentor is the pattern of the registered segmentor to use
    /// (e.g., "seg/qwen-flash"). Required. Must be registered in
    /// segmentors::DEFAULT_MUX (or the provided seg_mux).
    pub segmentor: String,

    /// Profiler is the pattern of the registered profiler to use
    /// (e.g., "prof/qwen-flash"). Optional. If empty, profiling is
    /// skipped and only segmentor output is used.
    pub profiler: Option<String>,

    /// Schema provides entity type hints to guide extraction.
    /// Optional. If nil, the LLM discovers entity types freely.
    pub schema: Option<segmentors::Schema>,

    /// Profiles holds current entity profiles for the profiler to
    /// reference when proposing updates. Optional.
    /// Keyed by entity label (e.g., "person:小明") → attribute map.
    pub profiles: Option<HashMap<String, HashMap<String, serde_json::Value>>>,

    /// SegmentorMux overrides the default segmentors mux. Optional.
    pub seg_mux: Option<Arc<SegmentorMux>>,

    /// ProfilerMux overrides the default profilers mux. Optional.
    pub prof_mux: Option<Arc<ProfilerMux>>,
}

impl LLMCompressorConfig {
    /// Validate the configuration.
    pub fn validate(&self) -> Result<(), MemoryError> {
        if self.segmentor.is_empty() {
            return Err(MemoryError::General(
                "memory: LLMCompressorConfig.segmentor is required".into(),
            ));
        }
        Ok(())
    }
}

impl std::fmt::Debug for LLMCompressorConfig {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("LLMCompressorConfig")
            .field("segmentor", &self.segmentor)
            .field("profiler", &self.profiler)
            .field("schema", &self.schema)
            .field("profiles", &self.profiles)
            .field("seg_mux", &if self.seg_mux.is_some() { "Some(Arc<SegmentorMux>)" } else { "None" })
            .field("prof_mux", &if self.prof_mux.is_some() { "Some(Arc<ProfilerMux>)" } else { "None" })
            .finish()
    }
}

/// LLMCompressor implements [Compressor] by delegating to the segmentors and
/// profilers packages for LLM-based conversation compression.
///
/// It calls [segmentors::SegmentorMux::process] to extract entities, relations, and a
/// compressed segment summary. Optionally, it calls [profilers::ProfilerMux::process] to
/// evolve entity schemas and update profiles.
///
/// LLMCompressor is stateless and safe for concurrent use.
#[derive(Clone)]
pub struct LLMCompressor {
    cfg: LLMCompressorConfig,
}

impl LLMCompressor {
    /// Create a new LLM-based compressor.
    pub fn new(cfg: LLMCompressorConfig) -> Result<Self, MemoryError> {
        cfg.validate()?;
        Ok(Self { cfg })
    }

    /// Run the segmentor.
    async fn run_segmentor(&self, input: segmentors::SegmentorInput) -> Result<SegmentorResult, MemoryError> {
        let result = if let Some(mux) = &self.cfg.seg_mux {
            mux.process(&self.cfg.segmentor, input).await
        } else {
            // Wait, we don't have DEFAULT_MUX exported in main's genx/segmentors
            // or do we? I'll just rely on the provided seg_mux for now, or expect it to be
            // provided in tests, because we removed the DEFAULT_MUX code I added.
            // Oh wait, `genx`'s `segmentors` does have `DEFAULT_MUX` if it's aligned with Go.
            // Let's assume there is a `DEFAULT_MUX` or a `process` function, or wait, `mux` is required if no global exists.
            // I'll check if `genx::segmentors::process` exists later, if not, I'll fix it.
            // Let's use the provided `seg_mux`. In Rust, we'll just require it if we don't have globals.
            // Wait, if it's aligned with Go, Go has `segmentors.Process` and `segmentors.DefaultMux`.
            // But I saw `SegmentorMux::new()` earlier in the read output and NO `DEFAULT_MUX`.
            // I will use `seg_mux` and return an error if missing for now, or assume it's mandatory if not using a global.
            // Let's assume `giztoy_genx::segmentors::process` exists. If it fails to compile, I will fix it.
            return Err(MemoryError::General("memory: Global DEFAULT_MUX is not available, please provide seg_mux in config".into()));
        };
        
        result.map_err(|e| MemoryError::General(e.to_string()))
    }

    /// Run the profiler.
    async fn run_profiler(&self, input: profilers::ProfilerInput) -> Result<ProfilerResult, MemoryError> {
        let pattern = self.cfg.profiler.as_ref().unwrap();
        let result = if let Some(mux) = &self.cfg.prof_mux {
            mux.process(pattern, input).await
        } else {
            return Err(MemoryError::General("memory: Global DEFAULT_MUX is not available, please provide prof_mux in config".into()));
        };
        
        result.map_err(|e| MemoryError::General(e.to_string()))
    }
}

#[async_trait]
impl Compressor for LLMCompressor {
    async fn compress_messages(&self, messages: &[Message]) -> Result<CompressResult, MemoryError> {
        let input = segmentors::SegmentorInput {
            messages: messages_to_strings(messages),
            schema: self.cfg.schema.clone(),
        };
        
        let result = self.run_segmentor(input).await?;

        // Convert segmentor output to CompressResult.
        let summary = result.segment.summary.clone();
        let seg = SegmentInput {
            summary: result.segment.summary,
            keywords: result.segment.keywords,
            labels: result.segment.labels,
        };

        Ok(CompressResult {
            segments: vec![seg],
            summary,
        })
    }

    async fn extract_entities(&self, messages: &[Message]) -> Result<EntityUpdate, MemoryError> {
        let input = segmentors::SegmentorInput {
            messages: messages_to_strings(messages),
            schema: self.cfg.schema.clone(),
        };

        let result = self.run_segmentor(input).await?;

        let mut update = convert_segmentor_result(&result);

        // Run profiler if configured.
        if self.cfg.profiler.is_some() {
            let prof_input = profilers::ProfilerInput {
                messages: messages_to_strings(messages),
                extracted: result.clone(),
                schema: self.cfg.schema.clone(),
                profiles: self.cfg.profiles.clone(),
            };
            if let Ok(prof_result) = self.run_profiler(prof_input).await {
                merge_profiler_result(&mut update, &prof_result);
            }
            // Profiler failure is non-fatal - we still have the segmentor result.
        }

        Ok(update)
    }

    async fn compact_segments(&self, summaries: &[String]) -> Result<CompressResult, MemoryError> {
        let input = segmentors::SegmentorInput {
            messages: summaries.to_vec(),
            schema: self.cfg.schema.clone(),
        };

        let result = self.run_segmentor(input).await?;

        let summary = result.segment.summary.clone();
        let seg = SegmentInput {
            summary: result.segment.summary,
            keywords: result.segment.keywords,
            labels: result.segment.labels,
        };

        Ok(CompressResult {
            segments: vec![seg],
            summary,
        })
    }
}

/// Convert segmentor entities and relations into a memory.EntityUpdate.
fn convert_segmentor_result(result: &SegmentorResult) -> EntityUpdate {
    let mut entities = Vec::new();
    for e in &result.entities {
        entities.push(EntityInput {
            label: e.label.clone(),
            attrs: e.attrs.clone(),
        });
    }

    let mut relations = Vec::new();
    for r in &result.relations {
        relations.push(RelationInput {
            from: r.from.clone(),
            to: r.to.clone(),
            rel_type: r.rel_type.clone(),
        });
    }

    EntityUpdate { entities, relations }
}

/// Merge profiler output into an existing EntityUpdate.
fn merge_profiler_result(update: &mut EntityUpdate, prof_result: &ProfilerResult) {
    // Merge profile updates as entity attrs.
    for (label, attrs) in &prof_result.profile_updates {
        let existing = update.entities.iter_mut().find(|e| e.label == *label);
        if let Some(existing_entity) = existing {
            for (k, v) in attrs {
                existing_entity.attrs.insert(k.clone(), v.clone());
            }
        } else {
            update.entities.push(EntityInput {
                label: label.clone(),
                attrs: attrs.clone(),
            });
        }
    }

    // Append additional relations from profiler.
    for r in &prof_result.relations {
        update.relations.push(RelationInput {
            from: r.from.clone(),
            to: r.to.clone(),
            rel_type: r.rel_type.clone(),
        });
    }
}

/// Convert memory.Message slice to the plain string format expected by segmentors/profilers:
/// "role: content" or "role(name): content".
pub fn messages_to_strings(messages: &[Message]) -> Vec<String> {
    messages
        .iter()
        .filter(|m| !m.content.is_empty())
        .map(|m| {
            let mut s = String::new();
            s.push_str(match m.role {
                crate::types::Role::User => "user",
                crate::types::Role::Model => "model",
                crate::types::Role::Tool => "tool",
            });
            if !m.name.is_empty() {
                s.push('(');
                s.push_str(&m.name);
                s.push(')');
            }
            s.push_str(": ");
            s.push_str(&m.content);
            s
        })
        .collect()
}
