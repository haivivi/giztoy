//! WebSocket-based realtime session.

use std::sync::Arc;

use async_trait::async_trait;
use base64::Engine;
use futures::stream::StreamExt;
use futures::SinkExt;
use serde_json::json;
use tokio::sync::{mpsc, Mutex};
use tokio_tungstenite::{connect_async, tungstenite::Message};
use tracing::{debug, error};

use crate::client::ClientConfig;
use crate::error::{Error, Result};
use crate::event::*;
use crate::session::Session;
use crate::types::*;

/// WebSocket-based realtime session.
pub struct WebSocketSession {
    write_tx: mpsc::Sender<Message>,
    event_rx: Mutex<mpsc::Receiver<Result<ServerEvent>>>,
    session_id: Mutex<Option<String>>,
    _read_handle: tokio::task::JoinHandle<()>,
    _write_handle: tokio::task::JoinHandle<()>,
}

impl WebSocketSession {
    /// Connects to the OpenAI Realtime API via WebSocket.
    pub(crate) async fn connect(config: Arc<ClientConfig>, model: &str) -> Result<Self> {
        // Build WebSocket URL with model query parameter
        let url = format!("{}?model={}", config.ws_url, model);
        debug!("Connecting to: {}", url);

        // Build request with headers
        let mut request = http::Request::builder()
            .uri(&url)
            .header("Authorization", format!("Bearer {}", config.api_key))
            .header("OpenAI-Beta", "realtime=v1");

        if let Some(ref org) = config.organization {
            request = request.header("OpenAI-Organization", org);
        }
        if let Some(ref project) = config.project {
            request = request.header("OpenAI-Project", project);
        }

        let request = request
            .header("Sec-WebSocket-Version", "13")
            .header("Sec-WebSocket-Key", generate_websocket_key())
            .header("Connection", "Upgrade")
            .header("Upgrade", "websocket")
            .header("Host", extract_host(&url).unwrap_or("api.openai.com"))
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
        let read_handle = tokio::spawn(read_loop(read, event_tx));

        Ok(Self {
            write_tx,
            event_rx: Mutex::new(event_rx),
            session_id: Mutex::new(None),
            _read_handle: read_handle,
            _write_handle: write_handle,
        })
    }

    /// Sends a JSON event to the server.
    async fn send_event(&self, event: serde_json::Value) -> Result<()> {
        let msg = Message::Text(event.to_string().into());
        debug!(
            "Sending event: {}",
            truncate_for_log(&event.to_string(), 500)
        );
        self.write_tx
            .send(msg)
            .await
            .map_err(|_| Error::SessionClosed)
    }
}

#[async_trait]
impl Session for WebSocketSession {
    async fn update_session(&self, config: &SessionConfig) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_SESSION_UPDATE,
            "session": config.to_json_value(),
        });
        self.send_event(event).await
    }

    async fn close(&self) -> Result<()> {
        let _ = self.write_tx.send(Message::Close(None)).await;
        Ok(())
    }

    fn session_id(&self) -> Option<String> {
        // Note: This is a synchronous method but we need to access the mutex.
        // In practice, this should be called from an async context.
        // For now, we'll use try_lock.
        self.session_id.try_lock().ok().and_then(|guard| guard.clone())
    }

    async fn append_audio(&self, audio: &[u8]) -> Result<()> {
        let encoded = base64::engine::general_purpose::STANDARD.encode(audio);
        self.append_audio_base64(&encoded).await
    }

    async fn append_audio_base64(&self, audio_base64: &str) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_BUFFER_APPEND,
            "audio": audio_base64,
        });
        self.send_event(event).await
    }

    async fn commit_input(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_BUFFER_COMMIT,
        });
        self.send_event(event).await
    }

    async fn clear_input(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_INPUT_AUDIO_BUFFER_CLEAR,
        });
        self.send_event(event).await
    }

    async fn add_user_message(&self, text: &str) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_CONVERSATION_ITEM_CREATE,
            "item": {
                "type": "message",
                "role": "user",
                "content": [{
                    "type": "input_text",
                    "text": text,
                }],
            },
        });
        self.send_event(event).await
    }

    async fn add_user_audio(&self, audio_base64: &str, transcript: Option<&str>) -> Result<()> {
        let mut content = json!({
            "type": "input_audio",
            "audio": audio_base64,
        });
        if let Some(t) = transcript {
            content["transcript"] = json!(t);
        }
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_CONVERSATION_ITEM_CREATE,
            "item": {
                "type": "message",
                "role": "user",
                "content": [content],
            },
        });
        self.send_event(event).await
    }

    async fn add_assistant_message(&self, text: &str) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_CONVERSATION_ITEM_CREATE,
            "item": {
                "type": "message",
                "role": "assistant",
                "content": [{
                    "type": "text",
                    "text": text,
                }],
            },
        });
        self.send_event(event).await
    }

    async fn add_function_call_output(&self, call_id: &str, output: &str) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_CONVERSATION_ITEM_CREATE,
            "item": {
                "type": "function_call_output",
                "call_id": call_id,
                "output": output,
            },
        });
        self.send_event(event).await
    }

    async fn truncate_item(
        &self,
        item_id: &str,
        content_index: i32,
        audio_end_ms: i32,
    ) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_CONVERSATION_ITEM_TRUNCATE,
            "item_id": item_id,
            "content_index": content_index,
            "audio_end_ms": audio_end_ms,
        });
        self.send_event(event).await
    }

    async fn delete_item(&self, item_id: &str) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_CONVERSATION_ITEM_DELETE,
            "item_id": item_id,
        });
        self.send_event(event).await
    }

    async fn create_response(&self, opts: Option<&ResponseCreateOptions>) -> Result<()> {
        let mut event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_RESPONSE_CREATE,
        });

        if let Some(opts) = opts {
            let mut response = serde_json::Map::new();

            if !opts.modalities.is_empty() {
                response.insert("modalities".to_string(), json!(opts.modalities));
            }
            if let Some(ref instructions) = opts.instructions {
                response.insert("instructions".to_string(), json!(instructions));
            }
            if let Some(ref voice) = opts.voice {
                response.insert("voice".to_string(), json!(voice));
            }
            if let Some(ref format) = opts.output_audio_format {
                response.insert("output_audio_format".to_string(), json!(format));
            }
            if !opts.tools.is_empty() {
                response.insert("tools".to_string(), json!(opts.tools));
            }
            if let Some(ref tool_choice) = opts.tool_choice {
                response.insert("tool_choice".to_string(), tool_choice.clone());
            }
            if let Some(temperature) = opts.temperature {
                response.insert("temperature".to_string(), json!(temperature));
            }
            if let Some(ref max_tokens) = opts.max_output_tokens {
                response.insert("max_output_tokens".to_string(), max_tokens.clone());
            }
            if let Some(ref conversation) = opts.conversation {
                response.insert("conversation".to_string(), json!(conversation));
            }
            if !opts.input.is_empty() {
                response.insert("input".to_string(), json!(opts.input));
            }

            if !response.is_empty() {
                event["response"] = serde_json::Value::Object(response);
            }
        }

        self.send_event(event).await
    }

    async fn cancel_response(&self) -> Result<()> {
        let event = json!({
            "event_id": generate_event_id(),
            "type": EVENT_TYPE_RESPONSE_CANCEL,
        });
        self.send_event(event).await
    }

    async fn recv(&mut self) -> Option<Result<ServerEvent>> {
        let mut rx = self.event_rx.lock().await;
        let result = rx.recv().await?;

        // Track session ID
        if let Ok(ref event) = result {
            if event.event_type == EVENT_TYPE_SESSION_CREATED {
                if let Some(ref session) = event.session {
                    let mut session_id = self.session_id.lock().await;
                    *session_id = Some(session.id.clone());
                }
            }
        }

        Some(result)
    }

    async fn send_raw(&self, event: serde_json::Value) -> Result<()> {
        self.send_event(event).await
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
    tx: mpsc::Sender<Result<ServerEvent>>,
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
fn parse_event(text: &str) -> Result<ServerEvent> {
    let mut event: ServerEvent = serde_json::from_str(text)?;
    event.raw = Some(text.as_bytes().to_vec());

    // Handle audio delta - the "delta" field contains base64 audio
    if event.event_type == EVENT_TYPE_RESPONSE_AUDIO_DELTA {
        if let Some(ref delta) = event.delta {
            event.audio_base64 = Some(delta.clone());
            if let Ok(decoded) = base64::engine::general_purpose::STANDARD.decode(delta) {
                event.audio = Some(decoded);
            }
        }
    }

    // Handle error event
    if event.event_type == EVENT_TYPE_ERROR {
        if let Some(ref error) = event.error {
            return Err(Error::Api(error.to_api_error()));
        }
    }

    Ok(event)
}

// Helper functions
fn generate_event_id() -> String {
    format!("evt_{}", &uuid::Uuid::new_v4().to_string()[..12])
}

fn generate_websocket_key() -> String {
    let mut key = [0u8; 16];
    for byte in &mut key {
        *byte = rand_byte();
    }
    base64::engine::general_purpose::STANDARD.encode(key)
}

fn rand_byte() -> u8 {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .subsec_nanos();
    (nanos % 256) as u8
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
