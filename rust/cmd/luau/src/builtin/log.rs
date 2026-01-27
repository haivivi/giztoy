//! Log builtin implementation.

use crate::runtime::Runtime;
use giztoy_luau::{Error, Type};

impl Runtime {
    /// Register __builtin.log
    pub fn register_log(&mut self) -> Result<(), Error> {
        self.state.register_func("__builtin_log", |state| {
            let n = state.get_top();
            let mut parts = Vec::with_capacity(n as usize);

            for i in 1..=n {
                let part = match state.get_type(i) {
                    Type::Nil => "nil".to_string(),
                    Type::Boolean => {
                        if state.to_boolean(i) {
                            "true".to_string()
                        } else {
                            "false".to_string()
                        }
                    }
                    Type::Number => format!("{}", state.to_number(i)),
                    Type::String => state.to_string(i).unwrap_or_default(),
                    Type::Table => "[table]".to_string(),
                    Type::Function => "[function]".to_string(),
                    _ => format!("[{}]", state.type_name(i)),
                };
                parts.push(part);
            }

            println!("{}", parts.join("\t"));
            0
        })?;

        self.state.get_global("__builtin_log")?;
        self.state.set_field(-2, "log")?;
        self.state.push_nil();
        self.state.set_global("__builtin_log")?;

        Ok(())
    }
}
