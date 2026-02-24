//! Generator multiplexer for pattern-based routing.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;

use crate::context::ModelContext;
use crate::error::{GenxError, Usage};
use crate::stream::Stream;
use crate::tool::FuncTool;
use crate::types::FuncCall;
use crate::Generator;

/// Generator multiplexer that routes requests to registered generators
/// based on pattern matching.
#[derive(Clone)]
pub struct Mux {
    routes: HashMap<String, Arc<dyn Generator>>,
}

impl Mux {
    pub fn new() -> Self {
        Self {
            routes: HashMap::new(),
        }
    }

    /// Register a generator for the given pattern.
    pub fn handle(
        &mut self,
        pattern: impl Into<String>,
        generator: Arc<dyn Generator>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        if self.routes.contains_key(&pattern) {
            return Err(GenxError::Other(anyhow::anyhow!(
                "generator already registered for {}",
                pattern,
            )));
        }
        self.routes.insert(pattern, generator);
        Ok(())
    }

    fn get(&self, pattern: &str) -> Result<&dyn Generator, GenxError> {
        self.routes
            .get(pattern)
            .map(|g| g.as_ref())
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!("generator not found for {}", pattern))
            })
    }

    /// Get a cloned Arc to the generator for the given pattern.
    pub fn get_arc(&self, pattern: &str) -> Result<Arc<dyn Generator>, GenxError> {
        self.routes
            .get(pattern)
            .cloned()
            .ok_or_else(|| {
                GenxError::Other(anyhow::anyhow!("generator not found for {}", pattern))
            })
    }
}

impl Default for Mux {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl Generator for Mux {
    async fn generate_stream(
        &self,
        model: &str,
        ctx: &dyn ModelContext,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let generator = self.get(model)?;
        generator.generate_stream(model, ctx).await
    }

    async fn invoke(
        &self,
        model: &str,
        ctx: &dyn ModelContext,
        tool: &FuncTool,
    ) -> Result<(Usage, FuncCall), GenxError> {
        let generator = self.get(model)?;
        generator.invoke(model, ctx, tool).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::context::ModelContextBuilder;

    struct MockGenerator {
        name: String,
    }

    #[async_trait]
    impl Generator for MockGenerator {
        async fn generate_stream(
            &self,
            _model: &str,
            _ctx: &dyn ModelContext,
        ) -> Result<Box<dyn Stream>, GenxError> {
            Err(GenxError::Other(anyhow::anyhow!(
                "mock: {}",
                self.name
            )))
        }

        async fn invoke(
            &self,
            _model: &str,
            _ctx: &dyn ModelContext,
            _tool: &FuncTool,
        ) -> Result<(Usage, FuncCall), GenxError> {
            Ok((
                Usage::default(),
                FuncCall::new(&self.name, "{}"),
            ))
        }
    }

    #[test]
    fn t5_1_register_and_find() {
        let mut mux = Mux::new();
        mux.handle(
            "qwen/turbo",
            Arc::new(MockGenerator { name: "turbo".into() }),
        )
        .unwrap();
        assert!(mux.get("qwen/turbo").is_ok());
    }

    #[test]
    fn t5_2_not_found() {
        let mux = Mux::new();
        assert!(mux.get("qwen/turbo").is_err());
    }

    #[test]
    fn t5_3_duplicate_registration() {
        let mut mux = Mux::new();
        mux.handle("a", Arc::new(MockGenerator { name: "a".into() }))
            .unwrap();
        let result = mux.handle("a", Arc::new(MockGenerator { name: "b".into() }));
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn t5_4_empty_mux_generate() {
        let mux = Mux::new();
        let ctx = ModelContextBuilder::new().build();
        let result = mux.generate_stream("anything", &ctx).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn t5_5_multiple_patterns() {
        let mut mux = Mux::new();
        mux.handle("a", Arc::new(MockGenerator { name: "alpha".into() }))
            .unwrap();
        mux.handle("b", Arc::new(MockGenerator { name: "beta".into() }))
            .unwrap();

        let ctx = ModelContextBuilder::new().build();
        let (_, call_a) = mux
            .invoke("a", &ctx, &FuncTool::new::<()>("f", "d"))
            .await
            .unwrap();
        let (_, call_b) = mux
            .invoke("b", &ctx, &FuncTool::new::<()>("f", "d"))
            .await
            .unwrap();
        assert_eq!(call_a.name, "alpha");
        assert_eq!(call_b.name, "beta");
    }

    #[tokio::test]
    async fn t5_generate_stream_through_mux() {
        let mut mux = Mux::new();
        mux.handle("test", Arc::new(MockGenerator { name: "test".into() }))
            .unwrap();
        let ctx = ModelContextBuilder::new().build();
        let result = mux.generate_stream("test", &ctx).await;
        // MockGenerator returns error from generate_stream
        assert!(result.is_err());
        assert!(result.err().unwrap().to_string().contains("mock: test"));
    }

    #[tokio::test]
    async fn t5_invoke_through_mux() {
        let mut mux = Mux::new();
        mux.handle("test", Arc::new(MockGenerator { name: "test".into() }))
            .unwrap();
        let ctx = ModelContextBuilder::new().build();
        let (_, call) = mux.invoke("test", &ctx, &FuncTool::new::<()>("f", "d")).await.unwrap();
        assert_eq!(call.name, "test");
    }
}
