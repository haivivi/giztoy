//! DashScope API client.

use std::sync::Arc;

use crate::{
    error::{Error, Result},
    realtime::RealtimeService,
};

/// Default WebSocket endpoint for Qwen-Omni-Realtime.
pub const DEFAULT_REALTIME_URL: &str = "wss://dashscope.aliyuncs.com/api-ws/v1/realtime";

/// Default HTTP endpoint.
pub const DEFAULT_HTTP_BASE_URL: &str = "https://dashscope.aliyuncs.com";

/// Default maximum number of retries.
pub const DEFAULT_MAX_RETRIES: u32 = 3;

/// DashScope API client.
///
/// The client provides access to DashScope API services.
///
/// # Example
///
/// ```rust,no_run
/// use giztoy_dashscope::Client;
///
/// let client = Client::new("your-api-key")?;
///
/// // Use services
/// let session = client.realtime().connect(&config).await?;
/// # Ok::<(), giztoy_dashscope::Error>(())
/// ```
pub struct Client {
    config: Arc<ClientConfig>,
}

/// Client configuration.
#[derive(Clone)]
pub(crate) struct ClientConfig {
    pub(crate) api_key: String,
    pub(crate) workspace_id: Option<String>,
    pub(crate) base_url: String,
    pub(crate) http_base_url: String,
    pub(crate) max_retries: u32,
}

impl Client {
    /// Creates a new DashScope API client.
    ///
    /// # Arguments
    ///
    /// * `api_key` - Your DashScope API key
    ///
    /// # Errors
    ///
    /// Returns an error if the API key is empty.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_dashscope::Client;
    ///
    /// let client = Client::new("your-api-key")?;
    /// # Ok::<(), giztoy_dashscope::Error>(())
    /// ```
    pub fn new(api_key: impl Into<String>) -> Result<Self> {
        ClientBuilder::new(api_key).build()
    }

    /// Creates a new client builder for more configuration options.
    pub fn builder(api_key: impl Into<String>) -> ClientBuilder {
        ClientBuilder::new(api_key)
    }

    /// Returns the configured API key.
    pub fn api_key(&self) -> &str {
        &self.config.api_key
    }

    /// Returns the configured base URL.
    pub fn base_url(&self) -> &str {
        &self.config.base_url
    }

    /// Returns the configured HTTP base URL.
    pub fn http_base_url(&self) -> &str {
        &self.config.http_base_url
    }

    /// Returns the realtime service.
    pub fn realtime(&self) -> RealtimeService {
        RealtimeService::new(self.config.clone())
    }

    /// Returns a reference to the client configuration.
    pub(crate) fn config(&self) -> &Arc<ClientConfig> {
        &self.config
    }
}

/// Builder for creating a DashScope API client.
pub struct ClientBuilder {
    api_key: String,
    workspace_id: Option<String>,
    base_url: String,
    http_base_url: String,
    max_retries: u32,
}

impl ClientBuilder {
    /// Creates a new client builder.
    pub fn new(api_key: impl Into<String>) -> Self {
        Self {
            api_key: api_key.into(),
            workspace_id: None,
            base_url: DEFAULT_REALTIME_URL.to_string(),
            http_base_url: DEFAULT_HTTP_BASE_URL.to_string(),
            max_retries: DEFAULT_MAX_RETRIES,
        }
    }

    /// Sets the workspace ID for resource isolation.
    pub fn workspace(mut self, workspace_id: impl Into<String>) -> Self {
        self.workspace_id = Some(workspace_id.into());
        self
    }

    /// Sets a custom WebSocket base URL.
    pub fn base_url(mut self, url: impl Into<String>) -> Self {
        self.base_url = url.into();
        self
    }

    /// Sets a custom HTTP base URL.
    pub fn http_base_url(mut self, url: impl Into<String>) -> Self {
        self.http_base_url = url.into();
        self
    }

    /// Sets the maximum number of retries for transient errors.
    pub fn max_retries(mut self, retries: u32) -> Self {
        self.max_retries = retries;
        self
    }

    /// Builds the client.
    pub fn build(self) -> Result<Client> {
        if self.api_key.is_empty() {
            return Err(Error::Config("api_key must be non-empty".to_string()));
        }

        Ok(Client {
            config: Arc::new(ClientConfig {
                api_key: self.api_key,
                workspace_id: self.workspace_id,
                base_url: self.base_url,
                http_base_url: self.http_base_url,
                max_retries: self.max_retries,
            }),
        })
    }
}
