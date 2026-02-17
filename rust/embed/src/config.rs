/// Builder-style configuration for embedder implementations.
pub struct EmbedConfig {
    pub model: String,
    pub dimension: usize,
    pub base_url: String,
}

impl EmbedConfig {
    pub fn with_model(mut self, model: &str) -> Self {
        self.model = model.to_string();
        self
    }

    pub fn with_dimension(mut self, dim: usize) -> Self {
        self.dimension = dim;
        self
    }

    pub fn with_base_url(mut self, url: &str) -> Self {
        self.base_url = url.to_string();
        self
    }
}
