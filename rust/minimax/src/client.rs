//! MiniMax API client.

use std::sync::Arc;

use super::{
    error::{Error, Result},
    file::FileService,
    http::HttpClient,
    image::ImageService,
    music::MusicService,
    speech::SpeechService,
    text::TextService,
    video::VideoService,
    voice::VoiceService,
};

/// Default MiniMax API base URL (China).
pub const DEFAULT_BASE_URL: &str = "https://api.minimaxi.com";

/// MiniMax API base URL for global/overseas users.
pub const BASE_URL_GLOBAL: &str = "https://api.minimaxi.chat";

/// Default maximum number of retries.
pub const DEFAULT_MAX_RETRIES: u32 = 3;

/// MiniMax API client.
///
/// The client provides access to all MiniMax API services.
///
/// # Example
///
/// ```rust,no_run
/// use giztoy::minimax::Client;
///
/// let client = Client::new("your-api-key")?;
///
/// // Use services
/// let response = client.speech().synthesize(&request).await?;
/// ```
pub struct Client {
    http: Arc<HttpClient>,
    config: ClientConfig,
}

/// Client configuration.
#[derive(Clone)]
struct ClientConfig {
    api_key: String,
    base_url: String,
    max_retries: u32,
}

impl Client {
    /// Creates a new MiniMax API client.
    ///
    /// # Arguments
    ///
    /// * `api_key` - Your MiniMax API key
    ///
    /// # Panics
    ///
    /// Panics if the API key is empty.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy::minimax::Client;
    ///
    /// let client = Client::new("your-api-key")?;
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

    /// Returns the text generation service.
    pub fn text(&self) -> TextService {
        TextService::new(self.http.clone())
    }

    /// Returns the speech synthesis service.
    pub fn speech(&self) -> SpeechService {
        SpeechService::new(self.http.clone())
    }

    /// Returns the voice management service.
    pub fn voice(&self) -> VoiceService {
        VoiceService::new(self.http.clone())
    }

    /// Returns the video generation service.
    pub fn video(&self) -> VideoService {
        VideoService::new(self.http.clone())
    }

    /// Returns the image generation service.
    pub fn image(&self) -> ImageService {
        ImageService::new(self.http.clone())
    }

    /// Returns the music generation service.
    pub fn music(&self) -> MusicService {
        MusicService::new(self.http.clone())
    }

    /// Returns the file management service.
    pub fn file(&self) -> FileService {
        FileService::new(self.http.clone())
    }

    /// Returns a reference to the internal HTTP client.
    pub fn http(&self) -> &Arc<HttpClient> {
        &self.http
    }
}

/// Builder for creating a MiniMax API client.
pub struct ClientBuilder {
    api_key: String,
    base_url: String,
    max_retries: u32,
}

impl ClientBuilder {
    /// Creates a new client builder.
    pub fn new(api_key: impl Into<String>) -> Self {
        Self {
            api_key: api_key.into(),
            base_url: DEFAULT_BASE_URL.to_string(),
            max_retries: DEFAULT_MAX_RETRIES,
        }
    }

    /// Sets a custom base URL for the API.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy::minimax::{Client, BASE_URL_GLOBAL};
    ///
    /// let client = Client::builder("your-api-key")
    ///     .base_url(BASE_URL_GLOBAL)
    ///     .build()?;
    /// ```
    pub fn base_url(mut self, url: impl Into<String>) -> Self {
        self.base_url = url.into();
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

        let http = HttpClient::new(self.base_url.clone(), self.api_key.clone(), self.max_retries)?;

        Ok(Client {
            http: Arc::new(http),
            config: ClientConfig {
                api_key: self.api_key,
                base_url: self.base_url,
                max_retries: self.max_retries,
            },
        })
    }
}
