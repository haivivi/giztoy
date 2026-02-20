//! HTTP client implementation for MiniMax API.

use std::time::Duration;

use bytes::Bytes;
use futures::{Stream, StreamExt};
use reqwest::{
    header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE, USER_AGENT},
    multipart, Client as ReqwestClient, Response, StatusCode,
};
use serde::{de::DeserializeOwned, Serialize};

use super::{
    error::{Error, Result},
    types::BaseResp,
};

/// HTTP client for MiniMax API.
pub struct HttpClient {
    client: ReqwestClient,
    base_url: String,
    api_key: String,
    max_retries: u32,
}

impl HttpClient {
    /// Creates a new HTTP client.
    pub fn new(base_url: String, api_key: String, max_retries: u32) -> Result<Self> {
        let client = ReqwestClient::builder()
            .timeout(Duration::from_secs(300))
            .build()?;

        Ok(Self {
            client,
            base_url,
            api_key,
            max_retries,
        })
    }

    /// Makes an HTTP request to the API with retry support.
    pub async fn request<T, R>(&self, method: &str, path: &str, body: Option<&T>) -> Result<R>
    where
        T: Serialize + ?Sized,
        R: DeserializeOwned,
    {
        let mut last_err = None;

        for attempt in 0..=self.max_retries {
            if attempt > 0 {
                // Exponential backoff: 1s, 2s, 4s, ...
                let backoff = Duration::from_secs(1 << (attempt - 1));
                tokio::time::sleep(backoff).await;
            }

            match self.do_request(method, path, body).await {
                Ok(result) => return Ok(result),
                Err(e) => {
                    if e.is_retryable() {
                        last_err = Some(e);
                        continue;
                    }
                    return Err(e);
                }
            }
        }

        Err(last_err.unwrap_or_else(|| Error::Other("max retries exceeded".to_string())))
    }

    /// Performs a single HTTP request.
    async fn do_request<T, R>(&self, method: &str, path: &str, body: Option<&T>) -> Result<R>
    where
        T: Serialize + ?Sized,
        R: DeserializeOwned,
    {
        let url = format!("{}{}", self.base_url, path);

        let mut request = match method {
            "GET" => self.client.get(&url),
            "POST" => self.client.post(&url),
            "PUT" => self.client.put(&url),
            "DELETE" => self.client.delete(&url),
            _ => return Err(Error::Other(format!("unsupported method: {}", method))),
        };

        request = request.headers(self.default_headers());

        if let Some(body) = body {
            request = request.json(body);
        }

        let response = request.send().await?;
        self.handle_response(response).await
    }

    /// Makes a streaming HTTP request.
    pub async fn request_stream<T>(
        &self,
        method: &str,
        path: &str,
        body: Option<T>,
    ) -> Result<impl Stream<Item = Result<Bytes>> + use<T>>
    where
        T: Serialize,
    {
        let url = format!("{}{}", self.base_url, path);

        let mut request = match method {
            "GET" => self.client.get(&url),
            "POST" => self.client.post(&url),
            _ => return Err(Error::Other(format!("unsupported method: {}", method))),
        };

        let mut headers = self.default_headers();
        headers.insert("Accept", HeaderValue::from_static("text/event-stream"));
        request = request.headers(headers);

        if let Some(ref body) = body {
            request = request.json(body);
        }

        let response = request.send().await?;

        if response.status() != StatusCode::OK {
            return Err(self.handle_error_response(response).await);
        }

        Ok(response.bytes_stream().map(|r| r.map_err(Error::from)))
    }

    /// Uploads a file using multipart form data.
    pub async fn upload_file<R>(
        &self,
        path: &str,
        file_bytes: Vec<u8>,
        filename: &str,
        fields: Vec<(&str, String)>,
    ) -> Result<R>
    where
        R: DeserializeOwned,
    {
        let url = format!("{}{}", self.base_url, path);

        let mut form = multipart::Form::new().part(
            "file",
            multipart::Part::bytes(file_bytes).file_name(filename.to_string()),
        );

        for (key, value) in fields {
            form = form.text(key.to_string(), value);
        }

        let mut headers = HeaderMap::new();
        headers.insert(
            AUTHORIZATION,
            HeaderValue::from_str(&format!("Bearer {}", self.api_key))
                .map_err(|e| Error::Other(e.to_string()))?,
        );
        headers.insert(
            USER_AGENT,
            HeaderValue::from_static("giztoy-minimax-rust/1.0"),
        );

        let response = self
            .client
            .post(&url)
            .headers(headers)
            .multipart(form)
            .send()
            .await?;

        self.handle_response(response).await
    }

    /// Returns default headers for API requests.
    fn default_headers(&self) -> HeaderMap {
        let mut headers = HeaderMap::new();
        headers.insert(
            AUTHORIZATION,
            HeaderValue::from_str(&format!("Bearer {}", self.api_key)).unwrap(),
        );
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        headers.insert(
            USER_AGENT,
            HeaderValue::from_static("giztoy-minimax-rust/1.0"),
        );
        headers
    }

    /// Handles the API response.
    async fn handle_response<R>(&self, response: Response) -> Result<R>
    where
        R: DeserializeOwned,
    {
        let status = response.status();
        let body = response.bytes().await?;

        if !status.is_success() {
            return Err(self.parse_error(&body, status.as_u16()));
        }

        // Check for API-level error in base_resp
        if let Ok(api_resp) = serde_json::from_slice::<ApiResponse>(&body) {
            if let Some(base_resp) = api_resp.base_resp {
                if base_resp.is_error() {
                    return Err(Error::api(
                        base_resp.status_code,
                        base_resp.status_msg,
                        status.as_u16(),
                    ));
                }
            }
        }

        serde_json::from_slice(&body).map_err(Error::from)
    }

    /// Handles an error response.
    async fn handle_error_response(&self, response: Response) -> Error {
        let status = response.status().as_u16();
        match response.bytes().await {
            Ok(body) => self.parse_error(&body, status),
            Err(e) => Error::Http(e),
        }
    }

    /// Parses an error response body.
    fn parse_error(&self, body: &[u8], http_status: u16) -> Error {
        if let Ok(api_resp) = serde_json::from_slice::<ApiResponse>(body) {
            if let Some(base_resp) = api_resp.base_resp {
                return Error::api(base_resp.status_code, base_resp.status_msg, http_status);
            }
        }

        Error::api(
            http_status as i32,
            String::from_utf8_lossy(body).to_string(),
            http_status,
        )
    }
}

/// API response wrapper.
#[derive(Debug, serde::Deserialize)]
struct ApiResponse {
    base_resp: Option<BaseResp>,
}

/// SSE (Server-Sent Events) reader.
pub(crate) struct SseReader<S> {
    stream: S,
    buffer: String,
}

impl<S> SseReader<S>
where
    S: Stream<Item = Result<Bytes>> + Unpin,
{
    pub fn new(stream: S) -> Self {
        Self {
            stream,
            buffer: String::new(),
        }
    }

    /// Reads the next SSE event.
    /// Returns (data, is_done).
    pub async fn read_event(&mut self) -> Result<Option<Vec<u8>>> {
        loop {
            // Check if we have a complete event in the buffer
            if let Some(event) = self.extract_event() {
                if event == "[DONE]" {
                    return Ok(None);
                }
                return Ok(Some(event.into_bytes()));
            }

            // Read more data from the stream
            match self.stream.next().await {
                Some(Ok(bytes)) => {
                    self.buffer.push_str(&String::from_utf8_lossy(&bytes));
                }
                Some(Err(e)) => return Err(e),
                None => return Ok(None),
            }
        }
    }

    /// Extracts a complete event from the buffer.
    fn extract_event(&mut self) -> Option<String> {
        // Find "data:" line followed by a double newline
        let mut search_pos = 0;

        while let Some(pos) = self.buffer[search_pos..].find("data:") {
            let abs_pos = search_pos + pos;
            let after_data = abs_pos + 5; // "data:" length

            // Skip whitespace after "data:"
            let content_start = self.buffer[after_data..]
                .chars()
                .take_while(|c| *c == ' ')
                .count()
                + after_data;

            // Find the end of this data line
            if let Some(newline_pos) = self.buffer[content_start..].find('\n') {
                let content_end = content_start + newline_pos;
                let content = self.buffer[content_start..content_end].trim();

                // Check if this is a complete event (followed by empty line or another event)
                let after_newline = content_end + 1;
                if after_newline >= self.buffer.len()
                    || self.buffer[after_newline..].starts_with('\n')
                    || self.buffer[after_newline..].starts_with("data:")
                {
                    let result = content.to_string();
                    self.buffer = self.buffer[after_newline..].trim_start_matches('\n').to_string();
                    return Some(result);
                }
            }

            search_pos = abs_pos + 1;
        }

        None
    }
}

/// Decodes hex-encoded audio data.
pub(crate) fn decode_hex_audio(hex_data: &str) -> Result<Vec<u8>> {
    let cleaned: String = hex_data.chars().filter(|c| !c.is_whitespace()).collect();
    hex::decode(&cleaned).map_err(Error::from)
}
