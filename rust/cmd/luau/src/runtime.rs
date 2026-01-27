//! Runtime for Luau script execution.

use giztoy_luau::{Error, State};
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

/// Runtime holds the state for Luau script execution.
pub struct Runtime {
    pub state: State,
    pub libs_dir: PathBuf,
    pub kvs: Arc<Mutex<HashMap<String, serde_json::Value>>>,
    pub loaded: Arc<Mutex<HashMap<String, bool>>>,
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
        })
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
