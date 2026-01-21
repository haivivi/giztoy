//! Doubao Speech API client.

use std::sync::Arc;

use crate::{
    asr::AsrService,
    error::{Error, Result},
    http::{AuthConfig, HttpClient},
    media::MediaService,
    meeting::MeetingService,
    podcast::PodcastService,
    translation::TranslationService,
    tts::TtsService,
    voice_clone::VoiceCloneService,
};

/// Default Doubao Speech API base URL.
pub const DEFAULT_BASE_URL: &str = "https://openspeech.bytedance.com";

/// Default Doubao Speech WebSocket URL.
pub const DEFAULT_WS_URL: &str = "wss://openspeech.bytedance.com";

/// Default maximum number of retries.
pub const DEFAULT_MAX_RETRIES: u32 = 3;

// Fixed App Keys for V3 APIs (documented constants)
// These are fixed values provided in official documentation, not user credentials.

/// Fixed X-Api-App-Key for Realtime dialogue API.
pub const APP_KEY_REALTIME: &str = "PlgvMymc7f3tQnJ6";

/// Fixed X-Api-App-Key for Podcast TTS API.
pub const APP_KEY_PODCAST: &str = "aGjiRDfUWi";

// V2/V3 API Resource IDs

/// TTS 1.0 (character-based)
pub const RESOURCE_TTS_V1: &str = "seed-tts-1.0";
/// TTS 1.0 (concurrent)
pub const RESOURCE_TTS_V1_CONCUR: &str = "seed-tts-1.0-concurr";
/// TTS 2.0 (character-based)
pub const RESOURCE_TTS_V2: &str = "seed-tts-2.0";
/// TTS 2.0 (concurrent)
pub const RESOURCE_TTS_V2_CONCUR: &str = "seed-tts-2.0-concurr";
/// Voice Clone 1.0
pub const RESOURCE_VOICE_CLONE_V1: &str = "seed-icl-1.0";
/// Voice Clone 2.0
pub const RESOURCE_VOICE_CLONE_V2: &str = "seed-icl-2.0";
/// ASR streaming (duration-based)
pub const RESOURCE_ASR_STREAM: &str = "volc.bigasr.sauc.duration";
/// ASR streaming 2.0
pub const RESOURCE_ASR_STREAM_V2: &str = "volc.seedasr.sauc.duration";
/// ASR file recognition
pub const RESOURCE_ASR_FILE: &str = "volc.bigasr.auc.duration";
/// Realtime dialogue
pub const RESOURCE_REALTIME: &str = "volc.speech.dialog";
/// Podcast synthesis
pub const RESOURCE_PODCAST: &str = "volc.service_type.10050";
/// Simultaneous translation
pub const RESOURCE_TRANSLATION: &str = "volc.megatts.simt";

/// Doubao Speech API client.
///
/// The client provides access to all Doubao Speech API services.
///
/// # Example
///
/// ```rust,no_run
/// use giztoy_doubaospeech::Client;
///
/// let client = Client::builder("your-app-id")
///     .api_key("your-api-key")
///     .build()?;
///
/// // Use TTS service
/// let response = client.tts().synthesize(&request).await?;
/// ```
pub struct Client {
    http: Arc<HttpClient>,
    config: Arc<ClientConfig>,
}

/// Client configuration.
#[derive(Clone)]
#[allow(dead_code)]
pub(crate) struct ClientConfig {
    pub(crate) app_id: String,
    pub(crate) base_url: String,
    pub(crate) ws_url: String,
    pub(crate) max_retries: u32,
    pub(crate) http: Arc<HttpClient>,
}

impl Client {
    /// Creates a new client builder.
    pub fn builder(app_id: impl Into<String>) -> ClientBuilder {
        ClientBuilder::new(app_id)
    }

    /// Returns the configured app ID.
    pub fn app_id(&self) -> &str {
        &self.config.app_id
    }

    /// Returns the configured base URL.
    pub fn base_url(&self) -> &str {
        &self.config.base_url
    }

    /// Returns the TTS (Text-to-Speech) service (V1 Classic API).
    pub fn tts(&self) -> TtsService {
        TtsService::new(self.http.clone())
    }

    /// Returns the ASR (Automatic Speech Recognition) service (V1 Classic API).
    pub fn asr(&self) -> AsrService {
        AsrService::new(self.http.clone())
    }

    /// Returns the Voice Clone service.
    pub fn voice_clone(&self) -> VoiceCloneService {
        VoiceCloneService::new(self.http.clone())
    }

    /// Returns the Meeting transcription service.
    pub fn meeting(&self) -> MeetingService {
        MeetingService::new(self.http.clone())
    }

    /// Returns the Podcast synthesis service.
    pub fn podcast(&self) -> PodcastService {
        PodcastService::new(self.http.clone())
    }

    /// Returns the Media processing service.
    pub fn media(&self) -> MediaService {
        MediaService::new(self.http.clone())
    }

    /// Returns the Translation service.
    pub fn translation(&self) -> TranslationService {
        TranslationService::new(self.http.clone())
    }

    /// Returns the Realtime service.
    pub fn realtime(&self) -> crate::realtime::RealtimeService {
        crate::realtime::RealtimeService::new(self.config.clone())
    }

    /// Returns a reference to the internal HTTP client.
    pub fn http(&self) -> &Arc<HttpClient> {
        &self.http
    }
}

/// Builder for creating a Doubao Speech API client.
pub struct ClientBuilder {
    app_id: String,
    access_token: Option<String>,
    api_key: Option<String>,
    access_key: Option<String>,
    app_key: Option<String>,
    cluster: Option<String>,
    user_id: String,
    base_url: String,
    ws_url: String,
    max_retries: u32,
}

impl ClientBuilder {
    /// Creates a new client builder.
    pub fn new(app_id: impl Into<String>) -> Self {
        Self {
            app_id: app_id.into(),
            access_token: None,
            api_key: None,
            access_key: None,
            app_key: None,
            cluster: None,
            user_id: "default_user".to_string(),
            base_url: DEFAULT_BASE_URL.to_string(),
            ws_url: DEFAULT_WS_URL.to_string(),
            max_retries: DEFAULT_MAX_RETRIES,
        }
    }

    /// Sets the Bearer Token for authentication.
    ///
    /// Header format: Authorization: Bearer;{token}
    pub fn bearer_token(mut self, token: impl Into<String>) -> Self {
        self.access_token = Some(token.into());
        self
    }

    /// Sets the simple API Key for authentication (recommended).
    ///
    /// API Key is from: https://console.volcengine.com/speech/new/setting/apikeys
    /// Header format: x-api-key: {apiKey}
    pub fn api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
        self
    }

    /// Sets the V2/V3 API Key for authentication.
    ///
    /// Header format:
    /// - X-Api-Access-Key: {accessKey}
    /// - X-Api-App-Key: {appKey}
    pub fn v2_api_key(mut self, access_key: impl Into<String>, app_key: impl Into<String>) -> Self {
        self.access_key = Some(access_key.into());
        self.app_key = Some(app_key.into());
        self
    }

    /// Sets the cluster name.
    ///
    /// Common clusters:
    /// - volcano_tts: TTS service
    /// - volcano_mega: TTS BigModel
    /// - volcano_icl: Voice Clone
    /// - volcengine_streaming_common: ASR streaming
    pub fn cluster(mut self, cluster: impl Into<String>) -> Self {
        self.cluster = Some(cluster.into());
        self
    }

    /// Sets the user ID.
    pub fn user_id(mut self, user_id: impl Into<String>) -> Self {
        self.user_id = user_id.into();
        self
    }

    /// Sets a custom base URL for the API.
    pub fn base_url(mut self, url: impl Into<String>) -> Self {
        self.base_url = url.into();
        self
    }

    /// Sets a custom WebSocket URL.
    pub fn ws_url(mut self, url: impl Into<String>) -> Self {
        self.ws_url = url.into();
        self
    }

    /// Sets the maximum number of retries for transient errors.
    pub fn max_retries(mut self, retries: u32) -> Self {
        self.max_retries = retries;
        self
    }

    /// Builds the client.
    pub fn build(self) -> Result<Client> {
        if self.app_id.is_empty() {
            return Err(Error::Config("app_id must be non-empty".to_string()));
        }

        // Check that at least one authentication method is provided
        if self.api_key.is_none()
            && self.access_token.is_none()
            && self.access_key.is_none()
        {
            return Err(Error::Config(
                "at least one authentication method must be provided".to_string(),
            ));
        }

        let auth = AuthConfig {
            app_id: self.app_id.clone(),
            access_token: self.access_token,
            api_key: self.api_key,
            access_key: self.access_key,
            app_key: self.app_key,
            cluster: self.cluster,
            user_id: self.user_id,
        };

        let http = HttpClient::new(
            self.base_url.clone(),
            self.ws_url.clone(),
            auth,
            self.max_retries,
        )?;

        let http = Arc::new(http);
        
        Ok(Client {
            config: Arc::new(ClientConfig {
                app_id: self.app_id,
                base_url: self.base_url,
                ws_url: self.ws_url,
                max_retries: self.max_retries,
                http: http.clone(),
            }),
            http,
        })
    }
}

/// Generates a unique request ID.
pub fn generate_req_id() -> String {
    uuid::Uuid::new_v4().to_string()
}
