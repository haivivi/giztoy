//! HTTP builtin implementation using reqwest + tokio.

use crate::runtime::Runtime;
use giztoy_luau::{Error, State};
use std::collections::HashMap;
use std::time::Duration;

/// HTTP client timeout
const TIMEOUT: Duration = Duration::from_secs(30);

/// HTTP request for async execution
#[derive(Debug, Clone)]
pub struct HttpRequest {
    pub url: String,
    pub method: String,
    pub headers: HashMap<String, String>,
    pub body: Option<String>,
}

/// HTTP response result
#[derive(Debug, Clone)]
pub struct HttpResponse {
    pub status: u16,
    pub headers: HashMap<String, String>,
    pub body: String,
    pub error: Option<String>,
}

impl Runtime {
    /// Register __builtin.http
    pub fn register_http(&mut self) -> Result<(), Error> {
        self.state.register_func("__builtin_http", |state| {
            builtin_http_sync(state)
        })?;

        self.state.get_global("__builtin_http")?;
        self.state.set_field(-2, "http")?;
        self.state.push_nil();
        self.state.set_global("__builtin_http")?;

        Ok(())
    }
}

/// Synchronous HTTP builtin (for compatibility)
fn builtin_http_sync(state: &State) -> i32 {
    // Parse request from Luau stack
    let request = match parse_http_request(state) {
        Ok(req) => req,
        Err(err) => {
            push_http_error(state, &err);
            return 1;
        }
    };

    // Execute HTTP synchronously using tokio's block_on
    // This is safe because we're in a single-threaded context
    let response = tokio::task::block_in_place(|| {
        tokio::runtime::Handle::current().block_on(execute_http_async(&request))
    });

    push_http_response(state, &response);
    1
}

/// Parse HTTP request from Luau stack
fn parse_http_request(state: &State) -> Result<HttpRequest, String> {
    if !state.is_table(1) {
        return Err("request must be a table".to_string());
    }

    // Read URL
    state.get_field(1, "url").ok();
    let url = state.to_string(-1).unwrap_or_default();
    state.pop(1);

    if url.is_empty() {
        return Err("url is required".to_string());
    }

    // Read method
    state.get_field(1, "method").ok();
    let method = state.to_string(-1).unwrap_or_else(|| "GET".to_string());
    state.pop(1);

    // Read body
    state.get_field(1, "body").ok();
    let body = state.to_string(-1);
    state.pop(1);

    // Read headers
    let mut headers: HashMap<String, String> = HashMap::new();
    state.get_field(1, "headers").ok();
    if state.is_table(-1) {
        state.push_nil();
        while state.next(-2) {
            if let (Some(key), Some(value)) = (state.to_string(-2), state.to_string(-1)) {
                headers.insert(key, value);
            }
            state.pop(1);
        }
    }
    state.pop(1);

    Ok(HttpRequest {
        url,
        method,
        headers,
        body,
    })
}

/// Execute HTTP request asynchronously using reqwest
pub async fn execute_http_async(request: &HttpRequest) -> HttpResponse {
    let client = reqwest::Client::builder()
        .timeout(TIMEOUT)
        .build();

    let client = match client {
        Ok(c) => c,
        Err(e) => {
            return HttpResponse {
                status: 0,
                headers: HashMap::new(),
                body: String::new(),
                error: Some(format!("failed to create client: {}", e)),
            };
        }
    };

    let method = match request.method.to_uppercase().as_str() {
        "GET" => reqwest::Method::GET,
        "POST" => reqwest::Method::POST,
        "PUT" => reqwest::Method::PUT,
        "PATCH" => reqwest::Method::PATCH,
        "DELETE" => reqwest::Method::DELETE,
        "HEAD" => reqwest::Method::HEAD,
        "OPTIONS" => reqwest::Method::OPTIONS,
        _ => {
            return HttpResponse {
                status: 0,
                headers: HashMap::new(),
                body: String::new(),
                error: Some(format!("unsupported HTTP method: {}", request.method)),
            };
        }
    };

    let mut req_builder = client.request(method, &request.url);

    // Set headers
    for (k, v) in &request.headers {
        req_builder = req_builder.header(k.as_str(), v.as_str());
    }

    // Set body
    if let Some(body) = &request.body {
        req_builder = req_builder.body(body.clone());
    }

    // Execute request
    match req_builder.send().await {
        Ok(resp) => {
            let status = resp.status().as_u16();

            // Collect response headers
            let mut resp_headers = HashMap::new();
            for (name, value) in resp.headers() {
                if let Ok(v) = value.to_str() {
                    resp_headers.insert(name.to_string(), v.to_string());
                }
            }

            // Read body
            let body = match resp.text().await {
                Ok(b) => b,
                Err(e) => {
                    eprintln!("[reqwest] warning: error reading response body: {}", e);
                    String::new()
                }
            };

            HttpResponse {
                status,
                headers: resp_headers,
                body,
                error: None,
            }
        }
        Err(e) => HttpResponse {
            status: 0,
            headers: HashMap::new(),
            body: String::new(),
            error: Some(format!("request failed: {}", e)),
        },
    }
}

/// Push HTTP error onto Luau stack
fn push_http_error(state: &State, error: &str) {
    state.new_table();
    state.push_number(0.0);
    state.set_field(-2, "status").ok();
    state.push_string(error).ok();
    state.set_field(-2, "err").ok();
}

/// Push HTTP response onto Luau stack
fn push_http_response(state: &State, response: &HttpResponse) {
    state.new_table();

    if let Some(err) = &response.error {
        state.push_number(0.0);
        state.set_field(-2, "status").ok();
        state.push_string(err).ok();
        state.set_field(-2, "err").ok();
        return;
    }

    state.push_number(response.status as f64);
    state.set_field(-2, "status").ok();

    state.push_string(&response.body).ok();
    state.set_field(-2, "body").ok();

    // Response headers
    state.new_table();
    for (k, v) in &response.headers {
        state.push_string(v).ok();
        state.set_field(-2, k).ok();
    }
    state.set_field(-2, "headers").ok();
}
