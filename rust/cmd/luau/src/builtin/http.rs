//! HTTP builtin implementation using reqwest + tokio.

use crate::runtime::Runtime;
use giztoy_luau::{Error, LuaStackOps, State, Thread};
use std::collections::HashMap;
use std::collections::HashSet;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
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

// Thread-local storage for async mode info
// This is used to pass async context into the Luau callback
thread_local! {
    static ASYNC_MODE: std::cell::RefCell<bool> = const { std::cell::RefCell::new(false) };
    static COMPLETED_TX: std::cell::RefCell<Option<tokio::sync::mpsc::UnboundedSender<(u64, HttpResponse)>>> = const { std::cell::RefCell::new(None) };
    static NEXT_REQ_ID: std::cell::RefCell<Option<Arc<AtomicU64>>> = const { std::cell::RefCell::new(None) };
    static PENDING_REQS: std::cell::RefCell<Option<Arc<Mutex<HashSet<u64>>>>> = const { std::cell::RefCell::new(None) };
}

impl Runtime {
    /// Register __builtin.http
    pub fn register_http(&mut self) -> Result<(), Error> {
        // Store async context in thread-local before registering
        ASYNC_MODE.with(|am| *am.borrow_mut() = self.async_mode);
        COMPLETED_TX.with(|tx| *tx.borrow_mut() = self.completed_tx.clone());
        NEXT_REQ_ID.with(|id| *id.borrow_mut() = Some(self.next_request_id.clone()));
        PENDING_REQS.with(|pr| *pr.borrow_mut() = Some(self.pending_reqs.clone()));
        
        self.state.register_func("__builtin_http", |state| {
            builtin_http(state)
        })?;

        self.state.get_global("__builtin_http")?;
        self.state.set_field(-2, "http")?;
        self.state.push_nil();
        self.state.set_global("__builtin_http")?;

        Ok(())
    }
    
    /// Update async mode in thread-local storage (call after changing async_mode)
    pub fn update_async_context(&self) {
        ASYNC_MODE.with(|am| *am.borrow_mut() = self.async_mode);
        COMPLETED_TX.with(|tx| *tx.borrow_mut() = self.completed_tx.clone());
        NEXT_REQ_ID.with(|id| *id.borrow_mut() = Some(self.next_request_id.clone()));
        PENDING_REQS.with(|pr| *pr.borrow_mut() = Some(self.pending_reqs.clone()));
    }
}

/// HTTP builtin that supports both sync and async modes
fn builtin_http(state: &State) -> i32 {
    // Parse request from Luau stack
    let request = match parse_http_request(state) {
        Ok(req) => req,
        Err(err) => {
            push_http_error(state, &err);
            return 1;
        }
    };

    // Check if we're in async mode and can yield
    let async_mode = ASYNC_MODE.with(|am| *am.borrow());
    let can_yield = state.is_yieldable();
    
    if async_mode && can_yield {
        // Async mode: start request in background and yield
        let completed_tx = COMPLETED_TX.with(|tx| tx.borrow().clone());
        let next_req_id = NEXT_REQ_ID.with(|id| id.borrow().clone());
        let pending_reqs = PENDING_REQS.with(|pr| pr.borrow().clone());
        
        if let (Some(tx), Some(req_id_counter), Some(pending)) = (completed_tx, next_req_id, pending_reqs) {
            // Generate request ID and add to pending set
            let req_id = req_id_counter.fetch_add(1, Ordering::SeqCst);
            {
                let mut pending_set = pending.lock().expect("pending_reqs mutex poisoned");
                pending_set.insert(req_id);
            }
            
            // Spawn async HTTP request
            tokio::spawn(async move {
                let response = execute_http_async(&request).await;
                let _ = tx.send((req_id, response));
            });
            
            // Push request ID onto stack (for debugging/tracking)
            state.push_number(req_id as f64);
            
            // Yield with 1 value (the request ID)
            return state.yield_results(1);
        }
    }

    // Sync mode: execute HTTP directly using tokio's block_on
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

/// Push HTTP error onto any Lua stack (State or Thread)
fn push_http_error(lua: &impl LuaStackOps, error: &str) {
    lua.new_table();
    lua.push_number(0.0);
    lua.set_field(-2, "status").ok();
    lua.push_string(error).ok();
    lua.set_field(-2, "err").ok();
}

/// Push HTTP response onto any Lua stack (State or Thread)
///
/// This generic function works with both `State` and `Thread` thanks to the
/// `LuaStackOps` trait, eliminating code duplication.
pub fn push_http_response(lua: &impl LuaStackOps, response: &HttpResponse) {
    lua.new_table();

    if let Some(err) = &response.error {
        lua.push_number(0.0);
        lua.set_field(-2, "status").ok();
        lua.push_string(err).ok();
        lua.set_field(-2, "err").ok();
        return;
    }

    lua.push_number(response.status as f64);
    lua.set_field(-2, "status").ok();

    lua.push_string(&response.body).ok();
    lua.set_field(-2, "body").ok();

    // Response headers
    lua.new_table();
    for (k, v) in &response.headers {
        lua.push_string(v).ok();
        lua.set_field(-2, k).ok();
    }
    lua.set_field(-2, "headers").ok();
}

/// Push HTTP response onto a Thread's stack (for async mode)
/// This is a convenience alias for push_http_response with Thread.
pub fn push_http_response_to_thread(thread: &Thread, response: &HttpResponse) {
    push_http_response(thread, response);
}
