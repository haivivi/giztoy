//! HTTP builtin implementation using std::process::Command to call curl.

use crate::runtime::Runtime;
use giztoy_luau::{Error, State};
use std::collections::HashMap;
use std::process::Command;

impl Runtime {
    /// Register __builtin.http
    pub fn register_http(&mut self) -> Result<(), Error> {
        self.state.register_func("__builtin_http", |state| {
            builtin_http(state)
        })?;

        self.state.get_global("__builtin_http")?;
        self.state.set_field(-2, "http")?;
        self.state.push_nil();
        self.state.set_global("__builtin_http")?;

        Ok(())
    }
}

fn builtin_http(state: &State) -> i32 {
    // Check if argument is a table
    if !state.is_table(1) {
        state.new_table();
        state.push_string("request must be a table").ok();
        state.set_field(-2, "err").ok();
        return 1;
    }

    // Read URL
    state.get_field(1, "url").ok();
    let url = state.to_string(-1).unwrap_or_default();
    state.pop(1);

    if url.is_empty() {
        state.new_table();
        state.push_string("url is required").ok();
        state.set_field(-2, "err").ok();
        return 1;
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

    // Build and execute request using curl
    let result = execute_curl_request(&url, &method, &headers, body.as_deref());

    match result {
        Ok((status, resp_body)) => {
            // Build response table
            state.new_table();

            state.push_number(status as f64);
            state.set_field(-2, "status").ok();

            state.push_string(&resp_body).ok();
            state.set_field(-2, "body").ok();

            // Empty headers table (curl doesn't easily provide response headers)
            state.new_table();
            state.set_field(-2, "headers").ok();

            1
        }
        Err(e) => {
            state.new_table();
            state.push_number(0.0);
            state.set_field(-2, "status").ok();
            state.push_string(&e).ok();
            state.set_field(-2, "err").ok();
            1
        }
    }
}

fn execute_curl_request(
    url: &str,
    method: &str,
    headers: &HashMap<String, String>,
    body: Option<&str>,
) -> Result<(u16, String), String> {
    let mut cmd = Command::new("curl");
    
    // Silent mode but show errors, include response code
    cmd.args(["-s", "-S", "-w", "\n%{http_code}"]);
    
    // Method
    cmd.args(["-X", method]);
    
    // Headers
    for (k, v) in headers {
        cmd.args(["-H", &format!("{}: {}", k, v)]);
    }
    
    // Body
    if let Some(body) = body {
        cmd.args(["-d", body]);
    }
    
    // URL
    cmd.arg(url);
    
    // Execute
    let output = cmd.output().map_err(|e| format!("failed to execute curl: {}", e))?;
    
    if !output.status.success() && output.stderr.len() > 0 {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("curl error: {}", stderr));
    }
    
    let stdout = String::from_utf8_lossy(&output.stdout);
    let lines: Vec<&str> = stdout.trim().rsplit('\n').collect();
    
    if lines.is_empty() {
        return Err("empty response from curl".to_string());
    }
    
    // Last line is status code
    let status: u16 = lines[0].parse().unwrap_or(0);
    
    // Rest is body (join back in reverse order, excluding last line)
    let body = if lines.len() > 1 {
        lines[1..].iter().rev().cloned().collect::<Vec<_>>().join("\n")
    } else {
        String::new()
    };
    
    Ok((status, body))
}
