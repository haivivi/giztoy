//! ModelContext provider multiplexer.

use std::collections::HashMap;
use std::sync::Arc;

use async_trait::async_trait;

use crate::context::ModelContext;
use crate::error::GenxError;

/// Provides a ModelContext for the given pattern.
#[async_trait]
pub trait ModelContextProvider: Send + Sync {
    async fn model_context(&self, pattern: &str)
        -> Result<Box<dyn ModelContext>, GenxError>;
}

/// Function-based ModelContextProvider.
pub struct ModelContextProviderFn<F>(pub F);

#[async_trait]
impl<F> ModelContextProvider for ModelContextProviderFn<F>
where
    F: Fn(&str) -> Result<Box<dyn ModelContext>, GenxError> + Send + Sync,
{
    async fn model_context(
        &self,
        pattern: &str,
    ) -> Result<Box<dyn ModelContext>, GenxError> {
        (self.0)(pattern)
    }
}

/// ModelContext provider multiplexer.
pub struct Mux {
    routes: HashMap<String, Arc<dyn ModelContextProvider>>,
}

impl Mux {
    pub fn new() -> Self {
        Self {
            routes: HashMap::new(),
        }
    }

    pub fn handle(
        &mut self,
        pattern: impl Into<String>,
        provider: Arc<dyn ModelContextProvider>,
    ) -> Result<(), GenxError> {
        let pattern = pattern.into();
        self.routes.insert(pattern, provider);
        Ok(())
    }

    pub async fn model_context(
        &self,
        pattern: &str,
    ) -> Result<Box<dyn ModelContext>, GenxError> {
        let provider = self.routes.get(pattern).ok_or_else(|| {
            GenxError::Other(anyhow::anyhow!(
                "model context provider not found for {}",
                pattern,
            ))
        })?;
        provider.model_context(pattern).await
    }
}

impl Default for Mux {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::context::{ModelContextBuilder, Prompt};

    struct StaticProvider {
        text: String,
    }

    #[async_trait]
    impl ModelContextProvider for StaticProvider {
        async fn model_context(
            &self,
            _pattern: &str,
        ) -> Result<Box<dyn ModelContext>, GenxError> {
            let mut b = ModelContextBuilder::new();
            b.prompt_text("system", &self.text);
            Ok(Box::new(b.build()))
        }
    }

    #[tokio::test]
    async fn t6_1_register_and_find() {
        let mut mux = Mux::new();
        mux.handle(
            "test",
            Arc::new(StaticProvider {
                text: "hello".into(),
            }),
        )
        .unwrap();
        let ctx = mux.model_context("test").await.unwrap();
        let prompts: Vec<&Prompt> = ctx.prompts().collect();
        assert_eq!(prompts[0].text, "hello");
    }

    #[tokio::test]
    async fn t6_2_not_found() {
        let mux = Mux::new();
        assert!(mux.model_context("missing").await.is_err());
    }

    #[tokio::test]
    async fn t6_3_multiple_providers() {
        let mut mux = Mux::new();
        mux.handle("a", Arc::new(StaticProvider { text: "alpha".into() }))
            .unwrap();
        mux.handle("b", Arc::new(StaticProvider { text: "beta".into() }))
            .unwrap();

        let ctx_a = mux.model_context("a").await.unwrap();
        let ctx_b = mux.model_context("b").await.unwrap();
        let pa: Vec<&Prompt> = ctx_a.prompts().collect();
        let pb: Vec<&Prompt> = ctx_b.prompts().collect();
        assert_eq!(pa[0].text, "alpha");
        assert_eq!(pb[0].text, "beta");
    }

    #[tokio::test]
    async fn t6_handle_func() {
        let mut mux = Mux::new();
        mux.handle(
            "fn_test",
            Arc::new(ModelContextProviderFn(|_pattern: &str| {
                let mut b = ModelContextBuilder::new();
                b.prompt_text("system", "from fn");
                Ok(Box::new(b.build()) as Box<dyn ModelContext>)
            })),
        )
        .unwrap();
        let ctx = mux.model_context("fn_test").await.unwrap();
        let prompts: Vec<&Prompt> = ctx.prompts().collect();
        assert_eq!(prompts[0].text, "from fn");
    }

    #[tokio::test]
    async fn t6_overwrite_allowed() {
        let mut mux = Mux::new();
        mux.handle("x", Arc::new(StaticProvider { text: "v1".into() })).unwrap();
        mux.handle("x", Arc::new(StaticProvider { text: "v2".into() })).unwrap();
        let ctx = mux.model_context("x").await.unwrap();
        let p: Vec<&Prompt> = ctx.prompts().collect();
        assert_eq!(p[0].text, "v2");
    }
}
