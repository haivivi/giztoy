//! JSON builtin implementation.

use crate::runtime::Runtime;
use giztoy_luau::{Error, LuaStackOps, State, Type};
use serde_json::Value;

impl Runtime {
    /// Register __builtin.json_encode and __builtin.json_decode
    pub fn register_json(&mut self) -> Result<(), Error> {
        // json_encode
        self.state.register_func("__builtin_json_encode", |state| {
            let value = lua_to_json(state, 1);
            match serde_json::to_string(&value) {
                Ok(s) => {
                    state.push_string(&s).ok();
                    1
                }
                Err(_) => {
                    state.push_nil();
                    1
                }
            }
        })?;

        self.state.get_global("__builtin_json_encode")?;
        self.state.set_field(-2, "json_encode")?;
        self.state.push_nil();
        self.state.set_global("__builtin_json_encode")?;

        // json_decode
        self.state.register_func("__builtin_json_decode", |state| {
            let s = state.to_string(1).unwrap_or_default();
            match serde_json::from_str::<Value>(&s) {
                Ok(value) => {
                    json_to_lua(state, &value);
                    1
                }
                Err(_) => {
                    state.push_nil();
                    1
                }
            }
        })?;

        self.state.get_global("__builtin_json_decode")?;
        self.state.set_field(-2, "json_decode")?;
        self.state.push_nil();
        self.state.set_global("__builtin_json_decode")?;

        Ok(())
    }
}

/// Convert Lua value to JSON value.
pub fn lua_to_json(state: &State, idx: i32) -> Value {
    match state.get_type(idx) {
        Type::Nil => Value::Null,
        Type::Boolean => Value::Bool(state.to_boolean(idx)),
        Type::Number => {
            let n = state.to_number(idx);
            if n.fract() == 0.0 {
                Value::Number(serde_json::Number::from(n as i64))
            } else {
                serde_json::Number::from_f64(n)
                    .map(Value::Number)
                    .unwrap_or(Value::Null)
            }
        }
        Type::String => Value::String(state.to_string(idx).unwrap_or_default()),
        Type::Table => lua_table_to_json(state, idx),
        _ => Value::Null,
    }
}

/// Convert Lua table to JSON value (array or object).
fn lua_table_to_json(state: &State, idx: i32) -> Value {
    // Convert negative index to absolute
    let abs_idx = if idx < 0 {
        state.get_top() + idx + 1
    } else {
        idx
    };

    // First pass: check if it's an array (all keys are consecutive integers starting from 1)
    let mut is_array = true;
    let mut max_idx: i32 = 0;
    let mut has_items = false;

    state.push_nil();
    while state.next(abs_idx) {
        has_items = true;
        if state.get_type(-2) != Type::Number {
            is_array = false;
        } else {
            let i = state.to_number(-2) as i32;
            if i > max_idx {
                max_idx = i;
            }
            // Check if it's a positive integer
            let n = state.to_number(-2);
            if n != n.floor() || n < 1.0 {
                is_array = false;
            }
        }
        state.pop(1); // Pop value, keep key for next iteration
    }

    // If array, use get_table to fetch values in order
    if is_array && max_idx > 0 {
        let mut arr = Vec::with_capacity(max_idx as usize);
        for i in 1..=max_idx {
            state.push_number(i as f64);
            state.get_table(abs_idx);
            arr.push(lua_to_json(state, -1));
            state.pop(1);
        }
        return Value::Array(arr);
    }

    // Convert to object - need to iterate again
    let mut obj = serde_json::Map::new();
    if has_items {
        state.push_nil();
        while state.next(abs_idx) {
            let key = match state.get_type(-2) {
                Type::String => state.to_string(-2).unwrap_or_default(),
                Type::Number => {
                    let n = state.to_number(-2);
                    if n.fract() == 0.0 {
                        format!("{}", n as i64)
                    } else {
                        format!("{}", n)
                    }
                },
                _ => {
                    state.pop(1);
                    continue;
                }
            };
            obj.insert(key, lua_to_json(state, -1));
            state.pop(1); // Pop value, keep key for next iteration
        }
    }

    Value::Object(obj)
}

/// Convert JSON value to Lua value.
pub fn json_to_lua(state: &State, value: &Value) {
    match value {
        Value::Null => state.push_nil(),
        Value::Bool(b) => state.push_boolean(*b),
        Value::Number(n) => state.push_number(n.as_f64().unwrap_or(0.0)),
        Value::String(s) => { state.push_string(s).ok(); }
        Value::Array(arr) => {
            state.new_table();
            let table_idx = state.get_top(); // Get absolute index of the table
            for (i, item) in arr.iter().enumerate() {
                // Push numeric key (1-based for Lua arrays)
                state.push_number((i + 1) as f64);
                // Push value (this may push multiple values for nested structures)
                json_to_lua(state, item);
                // set_table pops key and value: table[key] = value
                state.set_table(table_idx);
            }
        }
        Value::Object(obj) => {
            state.new_table();
            let table_idx = state.get_top(); // Get absolute index of the table
            for (k, v) in obj {
                // Push string key
                state.push_string(k).ok();
                // Push value (this may push multiple values for nested structures)
                json_to_lua(state, v);
                // set_table pops key and value: table[key] = value
                state.set_table(table_idx);
            }
        }
    }
}
