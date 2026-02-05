//! Minimal Luau runtime for script execution.
//!
//! This crate provides a reusable runtime for executing Luau scripts with:
//! - HTTP client (sync and async modes)
//! - JSON encoding/decoding
//! - Key-value storage
//! - Environment variable access
//! - Time utilities
//! - Module require with bytecode caching

pub mod builtin;

use giztoy_luau::{CoStatus, Error, LuaStackOps, OptLevel, State};
use std::collections::HashMap;
use std::collections::HashSet;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use tokio::sync::mpsc;

pub use builtin::http::{HttpRequest, HttpResponse};

/// Runtime holds the state for Luau script execution.
pub struct Runtime {
    pub state: State,
    pub libs_dir: PathBuf,
    pub kvs: Arc<Mutex<HashMap<String, serde_json::Value>>>,
    pub loaded: Arc<Mutex<HashMap<String, bool>>>,
    pub bytecode_cache: Arc<Mutex<HashMap<String, Vec<u8>>>>,

    // Async HTTP support
    pub async_mode: bool,
    pub next_request_id: Arc<AtomicU64>,
    pub pending_reqs: Arc<Mutex<HashSet<u64>>>,
    pub completed_tx: Option<mpsc::UnboundedSender<(u64, HttpResponse)>>,
    pub completed_rx: Option<mpsc::UnboundedReceiver<(u64, HttpResponse)>>,
}

impl Runtime {
    /// Create a new runtime.
    pub fn new(libs_dir: PathBuf) -> Result<Self, Error> {
        let state = State::new()?;
        state.open_libs();

        let (completed_tx, completed_rx) = mpsc::unbounded_channel();

        Ok(Runtime {
            state,
            libs_dir,
            kvs: Arc::new(Mutex::new(HashMap::new())),
            loaded: Arc::new(Mutex::new(HashMap::new())),
            bytecode_cache: Arc::new(Mutex::new(HashMap::new())),
            async_mode: false,
            next_request_id: Arc::new(AtomicU64::new(1)),
            pending_reqs: Arc::new(Mutex::new(HashSet::new())),
            completed_tx: Some(completed_tx),
            completed_rx: Some(completed_rx),
        })
    }

    /// Enable async HTTP mode.
    pub fn set_async_mode(&mut self, enabled: bool) {
        self.async_mode = enabled;
    }

    /// Generate next request ID and add to pending set.
    pub fn start_request(&self) -> u64 {
        let req_id = self.next_request_id.fetch_add(1, Ordering::SeqCst);
        let mut pending = self
            .pending_reqs
            .lock()
            .expect("pending_reqs mutex poisoned");
        pending.insert(req_id);
        req_id
    }

    /// Check if there are pending HTTP requests.
    pub fn has_pending_requests(&self) -> bool {
        let pending = self
            .pending_reqs
            .lock()
            .expect("pending_reqs mutex poisoned");
        !pending.is_empty()
    }

    /// Run a script in async mode with event loop.
    pub async fn run_async(&mut self, source: &str, chunkname: &str) -> Result<(), String> {
        // Compile script to bytecode
        let bytecode = self
            .state
            .compile(source, OptLevel::O2)
            .map_err(|e| format!("compile error: {}", e))?;

        // Create a new thread to run the script
        let thread = self
            .state
            .new_thread()
            .map_err(|e| format!("failed to create thread: {}", e))?;

        // Load bytecode onto thread's stack
        thread
            .load_bytecode(&bytecode, chunkname)
            .map_err(|e| format!("failed to load bytecode: {}", e))?;

        // Initial resume (start the script)
        let (mut status, _) = thread.resume(0);

        // Check for immediate error
        if status != CoStatus::Ok && status != CoStatus::Yield {
            let err_msg = thread
                .to_string(-1)
                .unwrap_or_else(|| "unknown error".to_string());
            return Err(format!("script error: {}", err_msg));
        }

        // Take ownership of the receiver for the event loop
        let mut completed_rx = self
            .completed_rx
            .take()
            .ok_or_else(|| "completed_rx already taken".to_string())?;

        // Event loop: process completed HTTP requests while coroutine is yielding
        // Continue while:
        // 1. Thread is yielding (waiting for async result), OR
        // 2. There are pending requests that will resume the thread
        while status == CoStatus::Yield {
            // Wait for a completed request or check if we should exit
            tokio::select! {
                biased;  // Prioritize completed requests

                Some((req_id, response)) = completed_rx.recv() => {
                    // Remove from pending set
                    {
                        let mut pending = self.pending_reqs.lock().expect("pending_reqs mutex poisoned");
                        pending.remove(&req_id);
                    }

                    // Push result onto thread's stack and resume
                    builtin::http::push_http_response_to_thread(&thread, &response);
                    let (new_status, _) = thread.resume(1);
                    status = new_status;

                    // Check for errors after resume
                    if status != CoStatus::Ok && status != CoStatus::Yield {
                        let err_msg = thread.to_string(-1).unwrap_or_else(|| "unknown error".to_string());
                        self.completed_rx = Some(completed_rx);
                        return Err(format!("thread error: {}", err_msg));
                    }
                }

                // Small delay only if there are pending requests (avoid busy loop)
                _ = tokio::time::sleep(tokio::time::Duration::from_millis(1)), if self.has_pending_requests() => {
                    // Continue waiting for pending requests
                }

                else => {
                    // No pending requests and channel empty - script yielded without pending HTTP
                    // This shouldn't happen in normal async HTTP usage, but handle gracefully
                    break;
                }
            }
        }

        // Restore the receiver
        self.completed_rx = Some(completed_rx);

        // Final status check
        if status != CoStatus::Ok {
            let err_msg = thread
                .to_string(-1)
                .unwrap_or_else(|| "unknown error".to_string());
            return Err(format!("runtime error: {}", err_msg));
        }

        Ok(())
    }

    /// Pre-compile all modules in the libs directory.
    pub fn precompile_modules(&mut self) -> Result<(), String> {
        let mut cache = self
            .bytecode_cache
            .lock()
            .expect("bytecode cache mutex poisoned");
        let mut errors = Vec::new();

        // Walk the libs directory
        if !self.libs_dir.exists() {
            return Ok(()); // No libs directory
        }

        fn walk_dir(
            dir: &PathBuf,
            base_dir: &PathBuf,
            state: &State,
            cache: &mut HashMap<String, Vec<u8>>,
            errors: &mut Vec<String>,
        ) {
            let entries = match std::fs::read_dir(dir) {
                Ok(e) => e,
                Err(e) => {
                    errors.push(format!("failed to read {:?}: {}", dir, e));
                    return;
                }
            };

            for entry in entries.flatten() {
                let path = entry.path();
                if path.is_dir() {
                    walk_dir(&path, base_dir, state, cache, errors);
                } else if path.extension().is_some_and(|e| e == "luau") {
                    // Calculate module name
                    let rel_path = path.strip_prefix(base_dir).unwrap_or(&path);
                    let mut module_name = rel_path.to_string_lossy().to_string();
                    module_name = module_name.trim_end_matches(".luau").to_string();
                    if module_name.ends_with("/init") {
                        module_name = module_name.trim_end_matches("/init").to_string();
                    }

                    // Read and compile
                    match std::fs::read_to_string(&path) {
                        Ok(source) => match state.compile(&source, OptLevel::O2) {
                            Ok(bytecode) => {
                                eprintln!(
                                    "[luau] precompiled: {} ({} bytes)",
                                    module_name,
                                    bytecode.len()
                                );
                                cache.insert(module_name, bytecode);
                            }
                            Err(e) => {
                                errors.push(format!("{:?}: compile error: {}", path, e));
                            }
                        },
                        Err(e) => {
                            errors.push(format!("{:?}: read error: {}", path, e));
                        }
                    }
                }
            }
        }

        walk_dir(
            &self.libs_dir.clone(),
            &self.libs_dir,
            &self.state,
            &mut cache,
            &mut errors,
        );

        if !errors.is_empty() {
            return Err(format!("compile errors:\n  {}", errors.join("\n  ")));
        }

        eprintln!("[luau] precompiled {} modules", cache.len());
        Ok(())
    }

    /// Get pre-compiled bytecode for a module.
    pub fn get_bytecode(&self, name: &str) -> Option<Vec<u8>> {
        let cache = self
            .bytecode_cache
            .lock()
            .expect("bytecode cache mutex poisoned");
        cache.get(name).cloned()
    }

    /// Register all builtin functions.
    pub fn register_builtins(&mut self) -> Result<(), Error> {
        // Create __builtin table
        self.state.new_table();

        // Register individual builtins
        self.register_http()?;
        self.register_json()?;
        self.register_kvs()?;
        self.register_log()?;
        self.register_env()?;
        self.register_time()?;

        // Set __builtin global
        self.state.set_global("__builtin")?;

        // Register require
        self.register_require()?;

        // Initialize __loaded table
        self.state.new_table();
        self.state.set_global("__loaded")?;

        Ok(())
    }

    /// Update async mode in thread-local storage (call after changing async_mode)
    pub fn update_async_context(&self) {
        builtin::http::update_async_context(
            self.async_mode,
            self.completed_tx.clone(),
            self.next_request_id.clone(),
            self.pending_reqs.clone(),
        );
    }
}
