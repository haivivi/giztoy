//! Environment variable builtin implementation.

use crate::Runtime;
use giztoy_luau::{Error, LuaStackOps};

impl Runtime {
    /// Register __builtin.env
    pub fn register_env(&mut self) -> Result<(), Error> {
        self.state.register_func("__builtin_env", |state| {
            let key = state.to_string(1).unwrap_or_default();
            if key.is_empty() {
                state.push_nil();
                return 1;
            }

            match std::env::var(&key) {
                Ok(value) => {
                    state.push_string(&value).ok();
                    1
                }
                Err(_) => {
                    state.push_nil();
                    1
                }
            }
        })?;

        self.state.get_global("__builtin_env")?;
        self.state.set_field(-2, "env")?;
        self.state.push_nil();
        self.state.set_global("__builtin_env")?;

        Ok(())
    }
}
