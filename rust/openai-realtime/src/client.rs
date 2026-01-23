//! Client for OpenAI Realtime API.

use std::sync::Arc;

use crate::error::{Error, Result};
use crate::types::*;
use crate::websocket::WebSocketSession;

/// Default WebSocket endpoint.
pub const DEFAULT_WEBSOCKET_URL: &str = "wss://api.openai.com/v1/realtime";

/// Default HTTP endpoint (for WebRTC session creation).
pub const DEFAULT_HTTP_URL: &str = "https://api.openai.com/v1/realtime";

/// OpenAI Realtime API client.
pub struct Client {
    config: Arc<ClientConfig>,
}

/// Client configuration.
#[derive(Debug, Clone)]
pub(crate) struct ClientConfig {
    pub api_key: String,
    pub organization: Option<String>,
    pub project: Option<String>,
    pub ws_url: String,
    pub http_url: String,
}

impl Client {
    /// Creates a new OpenAI Realtime client.
    ///
    /// The api_key is required and can be obtained from:
    /// https://platform.openai.com/api-keys
    pub fn new(api_key: impl Into<String>) -> Self {
        let api_key = api_key.into();
        if api_key.is_empty() {
            panic!("openai-realtime: API key is required");
        }

        Self {
            config: Arc::new(ClientConfig {
                api_key,
                organization: None,
                project: None,
                ws_url: DEFAULT_WEBSOCKET_URL.to_string(),
                http_url: DEFAULT_HTTP_URL.to_string(),
            }),
        }
    }

    /// Sets the organization ID for API requests.
    pub fn with_organization(mut self, org_id: impl Into<String>) -> Self {
        Arc::make_mut(&mut self.config).organization = Some(org_id.into());
        self
    }

    /// Sets the project ID for API requests.
    pub fn with_project(mut self, project_id: impl Into<String>) -> Self {
        Arc::make_mut(&mut self.config).project = Some(project_id.into());
        self
    }

    /// Sets the WebSocket URL.
    pub fn with_websocket_url(mut self, url: impl Into<String>) -> Self {
        Arc::make_mut(&mut self.config).ws_url = url.into();
        self
    }

    /// Sets the HTTP URL for WebRTC session creation.
    pub fn with_http_url(mut self, url: impl Into<String>) -> Self {
        Arc::make_mut(&mut self.config).http_url = url.into();
        self
    }

    /// Establishes a WebSocket connection to the Realtime API.
    /// This is suitable for server-side applications.
    pub async fn connect_websocket(
        &self,
        config: Option<&ConnectConfig>,
    ) -> Result<WebSocketSession> {
        let model = config
            .map(|c| c.model.as_str())
            .filter(|s| !s.is_empty())
            .unwrap_or(MODEL_GPT4O_REALTIME_PREVIEW);

        WebSocketSession::connect(self.config.clone(), model).await
    }

    // TODO: WebRTC support
    // pub async fn connect_webrtc(&self, config: Option<&ConnectConfig>) -> Result<WebRTCSession> { ... }
}

/// Builder for creating a Client with options.
pub struct ClientBuilder {
    api_key: String,
    organization: Option<String>,
    project: Option<String>,
    ws_url: Option<String>,
    http_url: Option<String>,
}

impl ClientBuilder {
    /// Creates a new client builder.
    pub fn new(api_key: impl Into<String>) -> Self {
        Self {
            api_key: api_key.into(),
            organization: None,
            project: None,
            ws_url: None,
            http_url: None,
        }
    }

    /// Sets the organization ID.
    pub fn organization(mut self, org_id: impl Into<String>) -> Self {
        self.organization = Some(org_id.into());
        self
    }

    /// Sets the project ID.
    pub fn project(mut self, project_id: impl Into<String>) -> Self {
        self.project = Some(project_id.into());
        self
    }

    /// Sets the WebSocket URL.
    pub fn websocket_url(mut self, url: impl Into<String>) -> Self {
        self.ws_url = Some(url.into());
        self
    }

    /// Sets the HTTP URL.
    pub fn http_url(mut self, url: impl Into<String>) -> Self {
        self.http_url = Some(url.into());
        self
    }

    /// Builds the client.
    pub fn build(self) -> Result<Client> {
        if self.api_key.is_empty() {
            return Err(Error::InvalidConfig("API key is required".to_string()));
        }

        Ok(Client {
            config: Arc::new(ClientConfig {
                api_key: self.api_key,
                organization: self.organization,
                project: self.project,
                ws_url: self.ws_url.unwrap_or_else(|| DEFAULT_WEBSOCKET_URL.to_string()),
                http_url: self.http_url.unwrap_or_else(|| DEFAULT_HTTP_URL.to_string()),
            }),
        })
    }
}
