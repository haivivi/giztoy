pub mod config;
pub mod dashscope;
pub mod embed;
pub mod error;
pub mod openai;
pub(crate) mod openai_compat;

pub use config::EmbedConfig;
pub use dashscope::DashScope;
pub use embed::Embedder;
pub use error::EmbedError;
pub use openai::OpenAI;
