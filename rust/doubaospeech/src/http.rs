//! HTTP client implementation for Doubao Speech API.

use std::time::Duration;

use bytes::Bytes;
use futures::{Stream, StreamExt};
use reqwest::{
    header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE, USER_AGENT},
    Client as ReqwestClient, Response, StatusCode,
};
use serde::{de::DeserializeOwned, Serialize};

use crate::error::{status_code, Error, Result};

/// HTTP client for Doubao Speech API.
pub struct HttpClient {
    client: ReqwestClient,
    base_url: String,
    ws_url: String,
    auth: AuthConfig,
    max_retries: u32,
}

/// Authentication configuration.
#[derive(Clone)]
pub struct AuthConfig {
    pub app_id: String,
    pub access_token: Option<String>,
    pub api_key: Option<String>,
    pub access_key: Option<String>,
    pub app_key: Option<String>,
    pub cluster: Option<String>,
    pub user_id: String,
}

impl HttpClient {
    /// Creates a new HTTP client.
    pub fn new(
        base_url: String,
        ws_url: String,
        auth: AuthConfig,
        max_retries: u32,
    ) -> Result<Self> {
        let client = ReqwestClient::builder()
            .timeout(Duration::from_secs(300))
            .build()?;

        Ok(Self {
            client,
            base_url,
            ws_url,
            auth,
            max_retries,
        })
    }

    /// Returns the base URL.
    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    /// Returns the WebSocket URL.
    pub fn ws_url(&self) -> &str {
        &self.ws_url
    }

    /// Returns the authentication configuration.
    pub fn auth(&self) -> &AuthConfig {
        &self.auth
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
    ) -> Result<impl Stream<Item = Result<Bytes>>>
    where
        T: Serialize,
    {
        let url = format!("{}{}", self.base_url, path);

        let mut request = match method {
            "GET" => self.client.get(&url),
            "POST" => self.client.post(&url),
            _ => return Err(Error::Other(format!("unsupported method: {}", method))),
        };

        request = request.headers(self.default_headers());

        if let Some(ref body) = body {
            request = request.json(body);
        }

        let response = request.send().await?;

        if response.status() != StatusCode::OK {
            return Err(self.handle_error_response(response).await);
        }

        Ok(response.bytes_stream().map(|r| r.map_err(Error::from)))
    }

    /// Returns default headers for V1 API requests.
    fn default_headers(&self) -> HeaderMap {
        let mut headers = HeaderMap::new();

        // Set authentication header
        if let Some(ref api_key) = self.auth.api_key {
            // Simple API Key (recommended)
            headers.insert("x-api-key", HeaderValue::from_str(api_key).unwrap());
        } else if let Some(ref token) = self.auth.access_token {
            // Bearer Token (note: format is "Bearer;{token}" not "Bearer {token}")
            let auth_value = format!("Bearer;{}", token);
            headers.insert(AUTHORIZATION, HeaderValue::from_str(&auth_value).unwrap());
        } else if let Some(ref access_key) = self.auth.access_key {
            // V2/V3 API Key
            headers.insert(
                "X-Api-Access-Key",
                HeaderValue::from_str(access_key).unwrap(),
            );
            if let Some(ref app_key) = self.auth.app_key {
                headers.insert("X-Api-App-Key", HeaderValue::from_str(app_key).unwrap());
            }
        }

        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        headers.insert(
            USER_AGENT,
            HeaderValue::from_static("giztoy-doubaospeech-rust/1.0"),
        );
        headers
    }

    /// Returns headers for V2/V3 API requests.
    pub fn v2_headers(&self, resource_id: Option<&str>) -> HeaderMap {
        let mut headers = HeaderMap::new();

        // Set App Key (AppID)
        headers.insert(
            "X-Api-App-Key",
            HeaderValue::from_str(&self.auth.app_id).unwrap(),
        );

        // Set Access Key (Bearer Token)
        if let Some(ref access_key) = self.auth.access_key {
            headers.insert(
                "X-Api-Access-Key",
                HeaderValue::from_str(access_key).unwrap(),
            );
        } else if let Some(ref token) = self.auth.access_token {
            headers.insert("X-Api-Access-Key", HeaderValue::from_str(token).unwrap());
        } else if let Some(ref api_key) = self.auth.api_key {
            headers.insert("x-api-key", HeaderValue::from_str(api_key).unwrap());
        }

        // Set Resource ID
        if let Some(resource_id) = resource_id {
            headers.insert(
                "X-Api-Resource-Id",
                HeaderValue::from_str(resource_id).unwrap(),
            );
        }

        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        headers.insert(
            USER_AGENT,
            HeaderValue::from_static("giztoy-doubaospeech-rust/1.0"),
        );

        headers
    }

    /// Handles the API response.
    async fn handle_response<R>(&self, response: Response) -> Result<R>
    where
        R: DeserializeOwned,
    {
        let status = response.status();
        let log_id = response
            .headers()
            .get("X-Tt-Logid")
            .and_then(|v| v.to_str().ok())
            .unwrap_or("")
            .to_string();

        let body = response.bytes().await?;

        if !status.is_success() {
            return Err(self.parse_error(&body, status.as_u16(), &log_id));
        }

        // Check for API-level error in response
        if let Ok(api_resp) = serde_json::from_slice::<ApiResponse>(&body) {
            if api_resp.code != status_code::SUCCESS && api_resp.code != status_code::ASR_SUCCESS {
                return Err(Error::Api {
                    code: api_resp.code,
                    message: api_resp.message,
                    req_id: api_resp.reqid,
                    log_id,
                    trace_id: String::new(),
                    http_status: status.as_u16(),
                });
            }
        }

        serde_json::from_slice(&body).map_err(Error::from)
    }

    /// Handles an error response.
    async fn handle_error_response(&self, response: Response) -> Error {
        let status = response.status().as_u16();
        let log_id = response
            .headers()
            .get("X-Tt-Logid")
            .and_then(|v| v.to_str().ok())
            .unwrap_or("")
            .to_string();

        match response.bytes().await {
            Ok(body) => self.parse_error(&body, status, &log_id),
            Err(e) => Error::Http(e),
        }
    }

    /// Parses an error response body.
    fn parse_error(&self, body: &[u8], http_status: u16, log_id: &str) -> Error {
        if let Ok(api_resp) = serde_json::from_slice::<ApiResponse>(body) {
            return Error::Api {
                code: api_resp.code,
                message: api_resp.message,
                req_id: api_resp.reqid,
                log_id: log_id.to_string(),
                trace_id: String::new(),
                http_status,
            };
        }

        Error::Api {
            code: http_status as i32,
            message: String::from_utf8_lossy(body).to_string(),
            req_id: String::new(),
            log_id: log_id.to_string(),
            trace_id: String::new(),
            http_status,
        }
    }

    /// Returns WebSocket authentication query parameters.
    pub fn ws_auth_params(&self) -> String {
        let mut params = format!("appid={}", self.auth.app_id);
        if let Some(ref token) = self.auth.access_token {
            params.push_str(&format!("&token={}", token));
        }
        if let Some(ref cluster) = self.auth.cluster {
            params.push_str(&format!("&cluster={}", cluster));
        }
        params
    }
}

/// API response wrapper.
#[derive(Debug, serde::Deserialize)]
struct ApiResponse {
    #[serde(default)]
    reqid: String,
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
}
