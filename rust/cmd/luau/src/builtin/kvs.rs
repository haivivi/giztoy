//! KVS builtin implementation.

use crate::builtin::json::{json_to_lua, lua_to_json};
use crate::runtime::Runtime;
use giztoy_luau::{Error, LuaStackOps};

impl Runtime {
    /// Register KVS builtins
    pub fn register_kvs(&mut self) -> Result<(), Error> {
        let kvs = self.kvs.clone();

        // kvs_get
        let kvs_get = kvs.clone();
        self.state.register_func("__builtin_kvs_get", move |state| {
            let key = state.to_string(1).unwrap_or_default();
            if key.is_empty() {
                state.push_nil();
                return 1;
            }

            let kvs = kvs_get.lock().expect("kvs mutex poisoned");
            match kvs.get(&key) {
                Some(value) => {
                    json_to_lua(state, value);
                    1
                }
                None => {
                    state.push_nil();
                    1
                }
            }
        })?;

        self.state.get_global("__builtin_kvs_get")?;
        self.state.set_field(-2, "kvs_get")?;
        self.state.push_nil();
        self.state.set_global("__builtin_kvs_get")?;

        // kvs_set
        let kvs_set = kvs.clone();
        self.state.register_func("__builtin_kvs_set", move |state| {
            let key = state.to_string(1).unwrap_or_default();
            if key.is_empty() {
                return 0;
            }

            let value = lua_to_json(state, 2);
            let mut kvs = kvs_set.lock().expect("kvs mutex poisoned");
            kvs.insert(key, value);
            0
        })?;

        self.state.get_global("__builtin_kvs_set")?;
        self.state.set_field(-2, "kvs_set")?;
        self.state.push_nil();
        self.state.set_global("__builtin_kvs_set")?;

        // kvs_del
        let kvs_del = kvs.clone();
        self.state.register_func("__builtin_kvs_del", move |state| {
            let key = state.to_string(1).unwrap_or_default();
            if !key.is_empty() {
                let mut kvs = kvs_del.lock().expect("kvs mutex poisoned");
                kvs.remove(&key);
            }
            0
        })?;

        self.state.get_global("__builtin_kvs_del")?;
        self.state.set_field(-2, "kvs_del")?;
        self.state.push_nil();
        self.state.set_global("__builtin_kvs_del")?;

        Ok(())
    }
}
