//! Profiler multiplexer.

use std::collections::HashMap;
use std::sync::Arc;

use crate::error::GenxError;

use super::types::{Profiler, ProfilerInput, ProfilerResult};

/// Profiler multiplexer that routes requests to registered implementations.
pub struct ProfilerMux {
    routes: HashMap<String, Arc<dyn Profiler>>,
}

impl ProfilerMux {
    pub fn new() -> Self {
        Self {
            routes: HashMap::new(),
        }
    }

    pub fn handle(
        &mut self,
        pattern: impl Into<String>,
        p: Arc<dyn Profiler>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        if self.routes.contains_key(&pattern) {
            return Err(GenxError::Other(anyhow::anyhow!(
                "profilers: profiler already registered for {}",
                pattern,
            )));
        }
        self.routes.insert(pattern, p);
        Ok(())
    }

    pub fn get(&self, pattern: &str) -> Result<&dyn Profiler, GenxError> {
        self.routes
            .get(pattern)
            .map(|p| p.as_ref())
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!(
                    "profilers: profiler not found for {}",
                    pattern,
                ))
            })
    }

    pub async fn process(
        &self,
        pattern: &str,
        input: ProfilerInput,
    ) -> Result<ProfilerResult, GenxError> {
        let p = self.get(pattern)?;
        p.process(input).await
    }
}

impl Default for ProfilerMux {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use async_trait::async_trait;

    struct FakeProfiler {
        name: String,
    }

    #[async_trait]
    impl Profiler for FakeProfiler {
        async fn process(&self, _input: ProfilerInput) -> Result<ProfilerResult, GenxError> {
            Ok(ProfilerResult {
                schema_changes: vec![],
                profile_updates: HashMap::new(),
                relations: vec![],
            })
        }
        fn model(&self) -> &str {
            &self.name
        }
    }

    #[tokio::test]
    async fn t12_5_mux_register_and_route() {
        let mut mux = ProfilerMux::new();
        mux.handle("test", Arc::new(FakeProfiler { name: "test".into() }))
            .unwrap();

        use crate::segmentors::{SegmentOutput, SegmentorResult};
        let input = ProfilerInput {
            messages: vec![],
            extracted: SegmentorResult {
                segment: SegmentOutput {
                    summary: "".into(),
                    keywords: vec![],
                    labels: vec![],
                },
                entities: vec![],
                relations: vec![],
            },
            schema: None,
            profiles: None,
        };
        let result = mux.process("test", input).await.unwrap();
        assert!(result.schema_changes.is_empty());
    }

    #[tokio::test]
    async fn t12_5_mux_not_found() {
        let mux = ProfilerMux::new();
        use crate::segmentors::{SegmentOutput, SegmentorResult};
        let input = ProfilerInput {
            messages: vec![],
            extracted: SegmentorResult {
                segment: SegmentOutput {
                    summary: "".into(),
                    keywords: vec![],
                    labels: vec![],
                },
                entities: vec![],
                relations: vec![],
            },
            schema: None,
            profiles: None,
        };
        assert!(mux.process("missing", input).await.is_err());
    }
}
