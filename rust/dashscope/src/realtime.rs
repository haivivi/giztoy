//! Realtime WebSocket API for Qwen-Omni-Realtime.

use std::sync::Arc;

use base64::Engine;
use futures::stream::{Stream, StreamExt};
use serde::{Deserialize, Serialize};
use serde_json::json;
use tokio::sync::mpsc;
use tokio_tungstenite::{connect_async, tungstenite::Message};
use tracing::{debug, error};

use crate::{
    client::ClientConfig,
    error::{Error, Result},
    event::*,
    types::*,
    MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST,
};

/// Realtime service for Qwen-Omni-Realtime API.
pub struct RealtimeService {
    config: Arc<ClientConfig>,
}

impl RealtimeService {
    pub(crate) fn new(config: Arc<ClientConfig>) -> Self {
        Self { config }
    }

    /// Connects to a realtime session.
    pub async fn connect(&self, realtime_config: &RealtimeConfig) -> Result<RealtimeSession> {
        let model = if realtime_config.model.is_empty() {
            MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST
        } else {
            &realtime_config.model
        };

        // Build WebSocket URL
        let url = format!("{}?model={}", self.config.base_url, model);
        debug!("Connecting to: {}", url);

        // Build request with headers
        let request = http::Request::builder()
            .uri(&url)
            .header("Authorization", format!("bearer {}", self.config.api_key));

        let request = if let Some(ref workspace_id) = self.config.workspace_id {
            request.header("X-DashScope-WorkSpace", workspace_id)
        } else {
            request
        };

        let request = request
            .header("Sec-WebSocket-Version", "13")
            .header("Sec-WebSocket-Key", generate_websocket_key())
            .header("Connection", "Upgrade")
            .header("Upgrade", "websocket")
            .header("Host", extract_host(&url).unwrap_or("dashscope.aliyuncs.com"))
            .body(())
            .map_err(|e| Error::Connection(format!("Failed to build request: {}", e)))?;

        // Connect
        let (ws_stream, _response) = connect_async(request)
            .await
            .map_err(|e| Error::Connection(format!("Failed to connect: {}", e)))?;

        let (write, read) = ws_stream.split();

        // Create channels
        let (event_tx, event_rx) = mpsc::channel(100);
        let (write_tx, write_rx) = mpsc::channel(100);

        // Spawn write task
        let write_handle = tokio::spawn(write_loop(write, write_rx));

        // Spawn read task
        let event_tx_clone = event_tx.clone();
        let read_handle = tokio::spawn(read_loop(read, event_tx_clone));

        Ok(RealtimeSession {
            config: realtime_config.clone(),
            event_rx,
            write_tx,
            _read_handle: read_handle,
            _write_handle: write_handle,
            session_id: None,
        })
    }
}

/// Realtime session configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeConfig {
    /// Model ID to use. Default: qwen-omni-turbo-realtime-latest
    #[serde(default)]
    pub model: String,
}

/// Options for creating a response.
#[derive(Debug, Clone, Default)]
pub struct ResponseCreateOptions {
    /// Messages for text input (DashScope-specific).
    pub messages: Option<Vec<SimpleMessage>>,
    /// Instructions override for this response.
    pub instructions: Option<String>,
    /// Output modalities for this response.
    pub modalities: Option<Vec<String>>,
}

/// Simple message with role and content.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SimpleMessage {
    pub role: String,
    pub content: String,
}

impl SimpleMessage {
    /// Creates a new text message.
    pub fn new(role: impl Into<String>, content: impl Into<String>) -> Self {
        Self {
            role: role.into(),
            content: content.into(),
        }
    }

    /// Creates a user message.
    pub fn user(content: impl Into<String>) -> Self {
        Self::new("user", content)
    }

    /// Creates an assistant message.
    pub fn assistant(content: impl Into<String>) -> Self {
        Self::new("assistant", content)
    }
}

/// Active realtime session.
pub struct RealtimeSession {
    config: RealtimeConfig,
    event_rx: mpsc::Receiver<Result<RealtimeEvent>>,
    write_tx: mpsc::Sender<Message>,
    _read_handle: tokio::task::JoinHandle<()>,
    _write_handle: tokio::task::JoinHandle<()>,
    session_id: Option<String>,
}

impl RealtimeSession {
    /// Returns the session configuration.
    pub fn config(&self) -> &RealtimeConfig {
        &self.config
    }

    /// Returns the session ID (if available).
    pub fn session_id(&self) -> Option<&str> {
        self.session_id.as_deref()
    }

    /// Updates the session configuration.
    pub async fn update_session(&self, config: &SessionConfig) -> Result<()> {
        let mut session_config = serde_json::Map::new();

        if let Some(ref modalities) = config.modalities {
            session_config.insert("modalities".to_string(), json!(modalities));
        }
        if let Some(ref voice) = config.voice {
            session_config.insert("voice".to_string(), json!(voice));
        }
        if let Some(ref format) = config.input_audio_format {
            session_config.insert("input_audio_format".to_string(), json!(format));
        }
        if let Some(ref format) = config.output_audio_format {
            session_config.insert("output_audio_format".to_string(), json!(format));
        }
        if let Some(ref instructions) = config.instructions {
            session_config.insert("instructions".to_string(), json!(instructions));
        }
        if let Some(enable) = config.enable_input_audio_transcription {
            if enable {
                let mut transcription = serde_json::Map::new();
                if let Some(ref model) = config.input_audio_transcription_model {
                    transcription.insert("model".to_string(), json!(model));
                }
                session_config.insert("input_audio_transcription".to_string(), json!(transcription));
            }
        }
        if let Some(ref td) = config.turn_detection {
            let mut turn_detection = serde_json::Map::new();
            if let Some(ref t) = td.detection_type {
                turn_detection.insert("type".to_string(), json!(t));
            }
            if let Some(threshold) = td.threshold {
                turn_detection.insert("threshold".to_string(), json!(threshold));
            }
            if let Some(prefix) = td.prefix_padding_ms {
                turn_detection.insert("prefix_padding_ms".to_string(), json!(prefix));
            }
            if let Some(silence) = td.silence_duration_ms {
                turn_detection.insert("silence_duration_ms".to_string(), json!(silence));
            }
            session_config.insert("turn_detection".to_string(), json!(turn_detection));
        }

        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_SESSION_UPDATE,
            "session": session_config,
        });

        self.send_event(event).await
    }

    /// Appends audio data to the input buffer.
    pub async fn append_audio(&self, audio: &[u8]) -> Result<()> {
        let encoded = base64::engine::general_purpose::STANDARD.encode(audio);
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_APPEND,
            "audio": encoded,
        });
        self.send_event(event).await
    }

    /// Appends base64-encoded audio data to the input buffer.
    pub async fn append_audio_base64(&self, audio_base64: &str) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_APPEND,
            "audio": audio_base64,
        });
        self.send_event(event).await
    }

    /// Appends an image frame to the input buffer.
    pub async fn append_image(&self, image: &[u8]) -> Result<()> {
        let encoded = base64::engine::general_purpose::STANDARD.encode(image);
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_IMAGE_APPEND,
            "image": encoded,
        });
        self.send_event(event).await
    }

    /// Commits the input audio buffer.
    pub async fn commit_input(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_COMMIT,
        });
        self.send_event(event).await
    }

    /// Clears the input audio buffer.
    pub async fn clear_input(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_CLEAR,
        });
        self.send_event(event).await
    }

    /// Creates a response.
    pub async fn create_response(&self, opts: Option<&ResponseCreateOptions>) -> Result<()> {
        let mut event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_RESPONSE_CREATE,
        });

        if let Some(opts) = opts {
            if let Some(ref messages) = opts.messages {
                event["messages"] = json!(messages);
            }

            let mut response = serde_json::Map::new();
            if let Some(ref instructions) = opts.instructions {
                response.insert("instructions".to_string(), json!(instructions));
            }
            if let Some(ref modalities) = opts.modalities {
                response.insert("modalities".to_string(), json!(modalities));
            }
            if !response.is_empty() {
                event["response"] = json!(response);
            }
        }

        self.send_event(event).await
    }

    /// Cancels the current response.
    pub async fn cancel_response(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_RESPONSE_CANCEL,
        });
        self.send_event(event).await
    }

    /// Finishes the session gracefully.
    pub async fn finish_session(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_SESSION_FINISH,
        });
        self.send_event(event).await
    }

    /// Sends a raw JSON event.
    pub async fn send_raw(&self, event: serde_json::Value) -> Result<()> {
        self.send_event(event).await
    }

    /// Returns a stream of events.
    pub fn events(&mut self) -> impl Stream<Item = Result<RealtimeEvent>> + '_ {
        futures::stream::poll_fn(move |cx| {
            self.event_rx.poll_recv(cx).map(|opt| opt)
        })
    }

    /// Receives the next event.
    pub async fn recv(&mut self) -> Option<Result<RealtimeEvent>> {
        self.event_rx.recv().await
    }

    /// Closes the session.
    pub async fn close(&self) -> Result<()> {
        let _ = self.write_tx.send(Message::Close(None)).await;
        Ok(())
    }

    async fn send_event(&self, event: serde_json::Value) -> Result<()> {
        let msg = Message::Text(event.to_string().into());
        debug!("Sending event: {}", truncate_for_log(&event.to_string(), 500));
        self.write_tx
            .send(msg)
            .await
            .map_err(|_| Error::SessionClosed)
    }
}

// Write loop task
async fn write_loop(
    mut write: futures::stream::SplitSink<
        tokio_tungstenite::WebSocketStream<
            tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>,
        >,
        Message,
    >,
    mut rx: mpsc::Receiver<Message>,
) {
    use futures::SinkExt;

    while let Some(msg) = rx.recv().await {
        if let Message::Close(_) = msg {
            let _ = write.close().await;
            break;
        }
        if let Err(e) = write.send(msg).await {
            error!("Write error: {}", e);
            break;
        }
    }
}

// Read loop task
async fn read_loop(
    mut read: futures::stream::SplitStream<
        tokio_tungstenite::WebSocketStream<
            tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>,
        >,
    >,
    tx: mpsc::Sender<Result<RealtimeEvent>>,
) {
    while let Some(result) = read.next().await {
        match result {
            Ok(Message::Text(text)) => {
                debug!("Received: {}", truncate_for_log(&text, 1000));
                let event = parse_event(&text);
                if tx.send(event).await.is_err() {
                    break;
                }
            }
            Ok(Message::Close(_)) => {
                debug!("WebSocket closed by server");
                break;
            }
            Ok(Message::Ping(data)) => {
                debug!("Received ping: {:?}", data);
            }
            Ok(_) => {}
            Err(e) => {
                error!("Read error: {}", e);
                let _ = tx.send(Err(Error::WebSocket(e))).await;
                break;
            }
        }
    }
}

// Parse event from JSON
fn parse_event(text: &str) -> Result<RealtimeEvent> {
    let raw: serde_json::Value = serde_json::from_str(text)?;
    let mut event = RealtimeEvent::default();

    // Extract event type
    if let Some(t) = raw.get("type").and_then(|v| v.as_str()) {
        event.event_type = t.to_string();
    }

    // Extract event ID
    if let Some(id) = raw.get("event_id").and_then(|v| v.as_str()) {
        event.event_id = Some(id.to_string());
    }

    // Check for DashScope "choices" format
    if let Some(choices) = raw.get("choices").and_then(|v| v.as_array()) {
        event.event_type = EVENT_TYPE_CHOICES_RESPONSE.to_string();
        
        if let Some(choice) = choices.first() {
            // Extract finish_reason
            if let Some(reason) = choice.get("finish_reason").and_then(|v| v.as_str()) {
                if reason != "null" {
                    event.finish_reason = Some(reason.to_string());
                }
            }

            // Extract message content
            if let Some(message) = choice.get("message") {
                if let Some(content) = message.get("content").and_then(|v| v.as_array()) {
                    for item in content {
                        // Text content
                        if let Some(text) = item.get("text").and_then(|v| v.as_str()) {
                            event.delta = Some(text.to_string());
                        }
                        // Audio content
                        if let Some(audio) = item.get("audio") {
                            if let Some(data) = audio.get("data").and_then(|v| v.as_str()) {
                                event.audio_base64 = Some(data.to_string());
                                if let Ok(decoded) = base64::engine::general_purpose::STANDARD.decode(data) {
                                    event.audio = Some(decoded);
                                }
                            }
                        }
                    }
                }
            }
        }
        return Ok(event);
    }

    // Standard event format parsing
    match event.event_type.as_str() {
        EVENT_TYPE_SESSION_CREATED | EVENT_TYPE_SESSION_UPDATED => {
            if let Some(session) = raw.get("session") {
                if let Ok(info) = serde_json::from_value(session.clone()) {
                    event.session = Some(info);
                }
            }
        }
        EVENT_TYPE_RESPONSE_CREATED => {
            if let Some(response) = raw.get("response") {
                if let Some(id) = response.get("id").and_then(|v| v.as_str()) {
                    event.response_id = Some(id.to_string());
                }
            }
        }
        EVENT_TYPE_RESPONSE_AUDIO_DELTA => {
            if let Some(delta) = raw.get("delta").and_then(|v| v.as_str()) {
                event.audio_base64 = Some(delta.to_string());
                if let Ok(decoded) = base64::engine::general_purpose::STANDARD.decode(delta) {
                    event.audio = Some(decoded);
                }
            }
        }
        EVENT_TYPE_RESPONSE_TRANSCRIPT_DELTA | EVENT_TYPE_RESPONSE_TEXT_DELTA => {
            if let Some(delta) = raw.get("delta").and_then(|v| v.as_str()) {
                event.delta = Some(delta.to_string());
            }
        }
        EVENT_TYPE_INPUT_AUDIO_TRANSCRIPTION_COMPLETED => {
            if let Some(transcript) = raw.get("transcript").and_then(|v| v.as_str()) {
                event.transcript = Some(transcript.to_string());
            }
        }
        EVENT_TYPE_RESPONSE_DONE => {
            if let Some(response) = raw.get("response") {
                if let Ok(info) = serde_json::from_value::<ResponseInfo>(response.clone()) {
                    event.response = Some(info.clone());
                    event.usage = info.usage;
                }
            }
        }
        EVENT_TYPE_ERROR => {
            if let Some(error) = raw.get("error") {
                if let Ok(err) = serde_json::from_value(error.clone()) {
                    event.error = Some(err);
                }
            }
        }
        _ => {}
    }

    Ok(event)
}

// Helper functions
fn generate_event_id() -> String {
    format!("event_{}", &uuid::Uuid::new_v4().to_string()[..12])
}

fn generate_websocket_key() -> String {
    use base64::Engine;
    let mut key = [0u8; 16];
    for byte in &mut key {
        *byte = rand_byte();
    }
    base64::engine::general_purpose::STANDARD.encode(key)
}

fn rand_byte() -> u8 {
    use std::time::{SystemTime, UNIX_EPOCH};
    let duration = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    ((duration.as_nanos() % 256) as u8).wrapping_add(duration.subsec_nanos() as u8)
}

fn extract_host(url: &str) -> Option<&str> {
    url.strip_prefix("wss://")
        .or_else(|| url.strip_prefix("ws://"))
        .and_then(|s| s.split('/').next())
        .and_then(|s| s.split('?').next())
}

fn truncate_for_log(s: &str, max_len: usize) -> String {
    if s.len() > max_len {
        format!("{}...", &s[..max_len])
    } else {
        s.to_string()
    }
}
