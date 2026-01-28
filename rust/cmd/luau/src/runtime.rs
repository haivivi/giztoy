//! Runtime for Luau script execution.

use crate::builtin::http::{HttpRequest, HttpResponse};
use giztoy_luau::{CoStatus, Error, OptLevel, State, Thread};
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use tokio::sync::mpsc;

/// Pending HTTP request with associated thread
pub struct PendingRequest {
    pub id: u64,
    pub result_rx: tokio::sync::oneshot::Receiver<HttpResponse>,
}

/// Runtime holds the state for Luau script execution.
pub struct Runtime {
    pub state: State,
    pub libs_dir: PathBuf,
    pub kvs: Arc<Mutex<HashMap<String, serde_json::Value>>>,
    pub loaded: Arc<Mutex<HashMap<String, bool>>>,
    pub bytecode_cache: Arc<Mutex<HashMap<String, Vec<u8>>>>,

    // Async HTTP support
    pub async_mode: bool,
    pub next_request_id: AtomicU64,
    pub pending_reqs: Arc<Mutex<HashMap<u64, PendingRequest>>>,
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
            next_request_id: AtomicU64::new(1),
            pending_reqs: Arc::new(Mutex::new(HashMap::new())),
            completed_tx: Some(completed_tx),
            completed_rx: Some(completed_rx),
        })
    }

    /// Enable async HTTP mode.
    pub fn set_async_mode(&mut self, enabled: bool) {
        self.async_mode = enabled;
    }

    /// Generate next request ID.
    pub fn next_request_id(&self) -> u64 {
        self.next_request_id.fetch_add(1, Ordering::SeqCst)
    }

    /// Check if there are pending HTTP requests.
    pub fn has_pending_requests(&self) -> bool {
        let pending = self.pending_reqs.lock().expect("pending_reqs mutex poisoned");
        !pending.is_empty()
    }

    /// Run a script in async mode with event loop.
    pub async fn run_async(&mut self, source: &str, chunkname: &str) -> Result<(), String> {
        // Compile script to bytecode
        let bytecode = self.state.compile(source, OptLevel::O2)
            .map_err(|e| format!("compile error: {}", e))?;

        // Create a new thread to run the script
        let thread = self.state.new_thread()
            .map_err(|e| format!("failed to create thread: {}", e))?;

        // Load bytecode onto thread's stack
        thread.load_bytecode(&bytecode, chunkname)
            .map_err(|e| format!("failed to load bytecode: {}", e))?;

        // Initial resume (start the script)
        let (mut status, _) = thread.resume(0);

        // Take ownership of the receiver for the event loop
        let mut completed_rx = self.completed_rx.take()
            .ok_or_else(|| "completed_rx already taken".to_string())?;

        // Event loop: poll for completed HTTP requests
        while status == CoStatus::Yield || self.has_pending_requests() {
            // Check for completed requests
            tokio::select! {
                Some((req_id, response)) = completed_rx.recv() => {
                    // Remove from pending map
                    {
                        let mut pending = self.pending_reqs.lock().expect("pending_reqs mutex poisoned");
                        pending.remove(&req_id);
                    }

                    // Push result onto thread's stack
                    crate::builtin::http::push_http_response_to_thread(&thread, &response);

                    // Resume the thread with 1 result
                    let (new_status, _) = thread.resume(1);
                    status = new_status;

                    if status != CoStatus::Ok && status != CoStatus::Yield {
                        let err_msg = thread.to_string(-1).unwrap_or_else(|| "unknown error".to_string());
                        return Err(format!("thread error: {}", err_msg));
                    }
                }
                _ = tokio::time::sleep(tokio::time::Duration::from_millis(1)), if self.has_pending_requests() => {
                    // Just a small delay to avoid busy loop
                }
                else => {
                    break;
                }
            }

            // Update status
            status = thread.status();
            if status == CoStatus::Ok {
                break;
            }
            if status != CoStatus::Yield {
                let err_msg = thread.to_string(-1).unwrap_or_else(|| "unknown error".to_string());
                return Err(format!("runtime error: {}", err_msg));
            }
        }

        // Restore the receiver (though we won't use it again)
        self.completed_rx = Some(completed_rx);

        // Check final status
        if status != CoStatus::Ok {
            let err_msg = thread.to_string(-1).unwrap_or_else(|| "unknown error".to_string());
            return Err(format!("runtime error: {}", err_msg));
        }

        Ok(())
    }
    
    /// Pre-compile all modules in the libs directory.
    pub fn precompile_modules(&mut self) -> Result<(), String> {
        let mut cache = self.bytecode_cache.lock().expect("bytecode cache mutex poisoned");
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
                } else if path.extension().map(|e| e == "luau").unwrap_or(false) {
                    // Calculate module name
                    let rel_path = path.strip_prefix(base_dir).unwrap_or(&path);
                    let mut module_name = rel_path.to_string_lossy().to_string();
                    module_name = module_name.trim_end_matches(".luau").to_string();
                    if module_name.ends_with("/init") {
                        module_name = module_name.trim_end_matches("/init").to_string();
                    }
                    
                    // Read and compile
                    match std::fs::read_to_string(&path) {
                        Ok(source) => {
                            match state.compile(&source, OptLevel::O2) {
                                Ok(bytecode) => {
                                    eprintln!("[luau] precompiled: {} ({} bytes)", module_name, bytecode.len());
                                    cache.insert(module_name, bytecode);
                                }
                                Err(e) => {
                                    errors.push(format!("{:?}: compile error: {}", path, e));
                                }
                            }
                        }
                        Err(e) => {
                            errors.push(format!("{:?}: read error: {}", path, e));
                        }
                    }
                }
            }
        }
        
        walk_dir(&self.libs_dir.clone(), &self.libs_dir, &self.state, &mut cache, &mut errors);
        
        if !errors.is_empty() {
            return Err(format!("compile errors:\n  {}", errors.join("\n  ")));
        }
        
        eprintln!("[luau] precompiled {} modules", cache.len());
        Ok(())
    }
    
    /// Get pre-compiled bytecode for a module.
    pub fn get_bytecode(&self, name: &str) -> Option<Vec<u8>> {
        let cache = self.bytecode_cache.lock().expect("bytecode cache mutex poisoned");
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
}
