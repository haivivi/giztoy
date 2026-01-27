//! Runtime for Luau script execution.

use giztoy_luau::{Error, OptLevel, State};
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

/// Runtime holds the state for Luau script execution.
pub struct Runtime {
    pub state: State,
    pub libs_dir: PathBuf,
    pub kvs: Arc<Mutex<HashMap<String, serde_json::Value>>>,
    pub loaded: Arc<Mutex<HashMap<String, bool>>>,
    pub bytecode_cache: Arc<Mutex<HashMap<String, Vec<u8>>>>,
}

impl Runtime {
    /// Create a new runtime.
    pub fn new(libs_dir: PathBuf) -> Result<Self, Error> {
        let state = State::new()?;
        state.open_libs();

        Ok(Runtime {
            state,
            libs_dir,
            kvs: Arc::new(Mutex::new(HashMap::new())),
            loaded: Arc::new(Mutex::new(HashMap::new())),
            bytecode_cache: Arc::new(Mutex::new(HashMap::new())),
        })
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
