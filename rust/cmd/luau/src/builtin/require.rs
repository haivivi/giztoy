//! Require builtin implementation.

use crate::runtime::Runtime;
use giztoy_luau::{Error, LuaStackOps, OptLevel};

impl Runtime {
    /// Register require function
    /// Uses pre-compiled bytecode when available for faster loading.
    pub fn register_require(&mut self) -> Result<(), Error> {
        // Convert to absolute path
        let libs_dir = std::fs::canonicalize(&self.libs_dir)
            .unwrap_or_else(|_| self.libs_dir.clone());
        let loaded = self.loaded.clone();
        let bytecode_cache = self.bytecode_cache.clone();

        self.state.register_func("require", move |state| {
            let name = state.to_string(1).unwrap_or_default();
            if name.is_empty() {
                state.push_nil();
                return 1;
            }

            // Validate module name to prevent path traversal
            if let Err(e) = validate_module_name(&name) {
                eprintln!("[luau] require error: {}", e);
                state.push_nil();
                return 1;
            }

            // Check if already loading (circular require)
            {
                let loaded_guard = loaded.lock().expect("loaded mutex poisoned");
                if loaded_guard.get(&name).copied().unwrap_or(false) {
                    // Return from cache - execute a Lua snippet to get from __loaded
                    drop(loaded_guard);
                    eprintln!("[luau] warning: circular require detected for module '{}'", name);
                    let escaped_name = escape_lua_string(&name);
                    let code = format!("return __loaded[\"{}\"]", escaped_name);
                    if state.do_string(&code).is_ok() {
                        return 1;
                    }
                    state.push_nil();
                    return 1;
                }
            }

            // Mark as loading
            {
                let mut loaded_guard = loaded.lock().expect("loaded mutex poisoned");
                loaded_guard.insert(name.clone(), true);
            }

            // Try to use pre-compiled bytecode first
            let bytecode = {
                let cache = bytecode_cache.lock().expect("bytecode cache mutex poisoned");
                cache.get(&name).cloned()
            };

            if let Some(bytecode) = bytecode {
                // Wrap the bytecode execution in a Lua snippet that caches the result
                // Since we can't easily duplicate stack values, we use a wrapper
                let escaped_name = escape_lua_string(&name);
                
                // Load the bytecode as a function
                if let Err(e) = state.load_bytecode(&bytecode, &name) {
                    eprintln!("[luau] require: failed to load bytecode for '{}': {}", name, e);
                    state.push_nil();
                    return 1;
                }
                
                // Execute the loaded chunk
                if let Err(e) = state.pcall(0, 1) {
                    eprintln!("[luau] require: failed to execute '{}': {}", name, e);
                    state.push_nil();
                    return 1;
                }
                
                // Cache the result using a Lua snippet
                // The result is on the stack, we need to cache it
                let cache_code = format!(
                    r#"
local result = ...
__loaded["{}"] = result
return result
"#,
                    escaped_name
                );
                
                // This is tricky - we need to pass the result to the cache code
                // For now, just set it directly if we can get a global temp
                state.set_global("__require_temp").ok();
                let final_code = format!(
                    r#"
local result = __require_temp
__require_temp = nil
__loaded["{}"] = result
return result
"#,
                    escaped_name
                );
                
                if let Err(e) = state.do_string(&final_code) {
                    eprintln!("[luau] require: failed to cache '{}': {}", name, e);
                    state.push_nil();
                    return 1;
                }
                
                return 1;
            }

            // Fallback: find and compile module file at runtime
            let candidates = vec![
                libs_dir.join(format!("{}.luau", name)),
                libs_dir.join(&name).join("init.luau"),
            ];

            let module_path = candidates.into_iter().find(|p| p.exists());

            let module_path = match module_path {
                Some(p) => p,
                None => {
                    eprintln!("require: module '{}' not found in {:?}", name, libs_dir);
                    state.push_nil();
                    return 1;
                }
            };

            // Read module source
            let source = match std::fs::read_to_string(&module_path) {
                Ok(s) => s,
                Err(e) => {
                    eprintln!("require: failed to read {:?}: {}", module_path, e);
                    state.push_nil();
                    return 1;
                }
            };

            // Wrap source to cache result in __loaded
            let escaped_name = escape_lua_string(&name);
            let wrapped_source = format!(
                r#"
local __module_result = (function()
{}
end)()
__loaded["{}"] = __module_result
return __module_result
"#,
                source,
                escaped_name
            );

            let path_str = module_path.to_string_lossy().to_string();

            // Execute module
            if let Err(e) = state.do_string_opt(&wrapped_source, &path_str, OptLevel::O2) {
                eprintln!("require: failed to execute {:?}: {}", module_path, e);
                state.push_nil();
                return 1;
            }

            1
        })?;

        Ok(())
    }
}

/// Validate module name to prevent path traversal attacks.
fn validate_module_name(name: &str) -> Result<(), String> {
    if name.is_empty() {
        return Err("module name cannot be empty".to_string());
    }

    // Reject path traversal sequences
    if name.contains("..") {
        return Err(format!("module name '{}' contains path traversal sequence", name));
    }

    // Reject absolute paths
    if name.starts_with('/') || name.starts_with('\\') {
        return Err(format!("module name '{}' starts with path separator", name));
    }

    // Reject Windows absolute paths
    if name.len() >= 2 && name.chars().nth(1) == Some(':') {
        return Err(format!("module name '{}' is an absolute path", name));
    }

    Ok(())
}

/// Escape a string for safe use in Lua string literals.
fn escape_lua_string(s: &str) -> String {
    s.replace('\\', "\\\\")
        .replace('"', "\\\"")
        .replace('\n', "\\n")
        .replace('\r', "\\r")
        .replace('\t', "\\t")
        .replace('\0', "\\0")
}
