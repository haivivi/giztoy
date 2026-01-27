//! Require builtin implementation.

use crate::runtime::Runtime;
use giztoy_luau::{Error, OptLevel};

impl Runtime {
    /// Register require function
    pub fn register_require(&mut self) -> Result<(), Error> {
        // Convert to absolute path
        let libs_dir = std::fs::canonicalize(&self.libs_dir)
            .unwrap_or_else(|_| self.libs_dir.clone());
        let loaded = self.loaded.clone();

        self.state.register_func("require", move |state| {
            let name = state.to_string(1).unwrap_or_default();
            if name.is_empty() {
                state.push_nil();
                return 1;
            }

            // Check if already loading (circular require)
            {
                let loaded_guard = loaded.lock().unwrap();
                if loaded_guard.get(&name).copied().unwrap_or(false) {
                    // Return from cache - execute a Lua snippet to get from __loaded
                    drop(loaded_guard);
                    let code = format!("return __loaded[\"{}\"]", name.replace('\"', "\\\""));
                    if state.do_string(&code).is_ok() {
                        return 1;
                    }
                    state.push_nil();
                    return 1;
                }
            }

            // Mark as loading
            {
                let mut loaded_guard = loaded.lock().unwrap();
                loaded_guard.insert(name.clone(), true);
            }

            // Find module file
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
            let wrapped_source = format!(
                r#"
local __module_result = (function()
{}
end)()
__loaded["{}"] = __module_result
return __module_result
"#,
                source,
                name.replace('\"', "\\\"")
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
