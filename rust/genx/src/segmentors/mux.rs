//! Segmentor multiplexer.

use std::collections::HashMap;
use std::sync::Arc;

use crate::error::GenxError;

use super::types::{Segmentor, SegmentorInput, SegmentorResult};

/// Segmentor multiplexer that routes requests to registered implementations.
pub struct SegmentorMux {
    routes: HashMap<String, Arc<dyn Segmentor>>,
}

impl SegmentorMux {
    pub fn new() -> Self {
        Self {
            routes: HashMap::new(),
        }
    }

    pub fn handle(
        &mut self,
        pattern: impl Into<String>,
        s: Arc<dyn Segmentor>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        if self.routes.contains_key(&pattern) {
            return Err(GenxError::Other(anyhow::anyhow!(
                "segmentors: segmentor already registered for {}",
                pattern,
            )));
        }
        self.routes.insert(pattern, s);
        Ok(())
    }

    pub fn get(&self, pattern: &str) -> Result<&dyn Segmentor, GenxError> {
        self.routes
            .get(pattern)
            .map(|s| s.as_ref())
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!(
                    "segmentors: segmentor not found for {}",
                    pattern,
                ))
            })
    }

    pub async fn process(
        &self,
        pattern: &str,
        input: SegmentorInput,
    ) -> Result<SegmentorResult, GenxError> {
        let s = self.get(pattern)?;
        s.process(input).await
    }
}

impl Default for SegmentorMux {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use async_trait::async_trait;

    struct FakeSegmentor {
        name: String,
    }

    #[async_trait]
    impl Segmentor for FakeSegmentor {
        async fn process(&self, _input: SegmentorInput) -> Result<SegmentorResult, GenxError> {
            Ok(SegmentorResult {
                segment: super::super::types::SegmentOutput {
                    summary: format!("from {}", self.name),
                    keywords: vec![],
                    labels: vec![],
                },
                entities: vec![],
                relations: vec![],
            })
        }
        fn model(&self) -> &str {
            &self.name
        }
    }

    #[tokio::test]
    async fn t11_1_register_and_process() {
        let mut mux = SegmentorMux::new();
        mux.handle("test", Arc::new(FakeSegmentor { name: "test".into() }))
            .unwrap();

        let result = mux
            .process(
                "test",
                SegmentorInput {
                    messages: vec![],
                    schema: None,
                },
            )
            .await
            .unwrap();
        assert_eq!(result.segment.summary, "from test");
    }

    #[tokio::test]
    async fn t11_2_not_found() {
        let mux = SegmentorMux::new();
        let result = mux
            .process(
                "missing",
                SegmentorInput {
                    messages: vec![],
                    schema: None,
                },
            )
            .await;
        assert!(result.is_err());
    }

    #[test]
    fn t11_3_duplicate_registration() {
        let mut mux = SegmentorMux::new();
        mux.handle("a", Arc::new(FakeSegmentor { name: "a".into() }))
            .unwrap();
        let r = mux.handle("a", Arc::new(FakeSegmentor { name: "a2".into() }));
        assert!(r.is_err());
    }
}
