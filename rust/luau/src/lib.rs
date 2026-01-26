//! Luau scripting language bindings for Rust.
//!
//! This crate provides a safe Rust interface to the Luau scripting language,
//! using the C wrapper for FFI binding.
//!
//! # Example
//!
//! ```rust
//! use giztoy_luau::State;
//!
//! let state = State::new().unwrap();
//! state.open_libs();
//! state.do_string("x = 1 + 2").unwrap();
//! ```

mod ffi;

use std::collections::HashMap;
use std::ffi::{CStr, CString};
use std::os::raw::c_char;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::RwLock;
use thiserror::Error;

// Global function registry
static GLOBAL_FUNCS: RwLock<Option<HashMap<u64, RustFuncEntry>>> = RwLock::new(None);
static GLOBAL_FUNC_NEXT_ID: AtomicU64 = AtomicU64::new(1);

/// Entry in the global function registry
struct RustFuncEntry {
    func: Box<dyn Fn(&State) -> i32 + Send + Sync>,
    state_ptr: *mut ffi::LuauState,
}

// Safety: RustFuncEntry is Send+Sync because:
// - func is Box<dyn Fn + Send + Sync>
// - state_ptr is only used for comparison, not dereferenced across threads
unsafe impl Send for RustFuncEntry {}
unsafe impl Sync for RustFuncEntry {}

/// Error types returned by Luau operations.
#[derive(Error, Debug, Clone)]
pub enum Error {
    #[error("Luau compilation error: {0}")]
    Compile(String),

    #[error("Luau runtime error: {0}")]
    Runtime(String),

    #[error("Luau memory error")]
    Memory,

    #[error("Invalid operation")]
    Invalid,

    #[error("Null pointer error")]
    NullPointer,
}

/// Result type for Luau operations.
pub type Result<T> = std::result::Result<T, Error>;

/// Luau value types.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(i32)]
pub enum Type {
    Nil = 0,
    Boolean = 1,
    Number = 2,
    String = 3,
    Table = 4,
    Function = 5,
    Userdata = 6,
    Thread = 7,
    Buffer = 8,
    Vector = 9,
}

impl From<i32> for Type {
    fn from(value: i32) -> Self {
        match value {
            0 => Type::Nil,
            1 => Type::Boolean,
            2 => Type::Number,
            3 => Type::String,
            4 => Type::Table,
            5 => Type::Function,
            6 => Type::Userdata,
            7 => Type::Thread,
            8 => Type::Buffer,
            9 => Type::Vector,
            _ => Type::Nil,
        }
    }
}

/// Compiler optimization level.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(i32)]
pub enum OptLevel {
    None = 0,
    O1 = 1,
    O2 = 2,
}

/// Represents a Luau virtual machine state.
pub struct State {
    ptr: *mut ffi::LuauState,
    func_ids: Vec<u64>,
    callback_set: AtomicBool,
}

// State is Send but not Sync (not thread-safe)
unsafe impl Send for State {}

impl State {
    /// Create a new Luau state.
    pub fn new() -> Result<Self> {
        let ptr = unsafe { ffi::luau_new() };
        if ptr.is_null() {
            return Err(Error::Memory);
        }
        
        // Initialize global registry if needed
        {
            let mut guard = GLOBAL_FUNCS.write().unwrap();
            if guard.is_none() {
                *guard = Some(HashMap::new());
            }
        }
        
        Ok(State {
            ptr,
            func_ids: Vec::new(),
            callback_set: AtomicBool::new(false),
        })
    }

    /// Open standard libraries.
    pub fn open_libs(&self) {
        unsafe { ffi::luau_openlibs(self.ptr) };
    }

    /// Execute a Luau script string.
    pub fn do_string(&self, source: &str) -> Result<()> {
        self.do_string_opt(source, "", OptLevel::O2)
    }

    /// Execute a Luau script string with options.
    pub fn do_string_opt(&self, source: &str, chunkname: &str, opt: OptLevel) -> Result<()> {
        let c_source = CString::new(source).map_err(|_| Error::Invalid)?;
        let c_chunkname = if chunkname.is_empty() {
            std::ptr::null()
        } else {
            CString::new(chunkname)
                .map_err(|_| Error::Invalid)?
                .into_raw()
        };

        let result = unsafe {
            ffi::luau_dostring(
                self.ptr,
                c_source.as_ptr(),
                source.len(),
                c_chunkname,
                opt as i32,
            )
        };

        // Free the chunkname if we allocated it
        if !c_chunkname.is_null() {
            unsafe { drop(CString::from_raw(c_chunkname as *mut c_char)) };
        }

        self.check_error(result)
    }

    /// Compile a Luau script to bytecode.
    pub fn compile(&self, source: &str, opt: OptLevel) -> Result<Vec<u8>> {
        let c_source = CString::new(source).map_err(|_| Error::Invalid)?;
        let mut bytecode: *mut c_char = std::ptr::null_mut();
        let mut bytecode_len: usize = 0;

        let result = unsafe {
            ffi::luau_compile(
                c_source.as_ptr(),
                source.len(),
                opt as i32,
                &mut bytecode,
                &mut bytecode_len,
            )
        };

        if result != 0 || bytecode.is_null() {
            return Err(Error::Compile("compilation failed".to_string()));
        }

        let slice = unsafe { std::slice::from_raw_parts(bytecode as *const u8, bytecode_len) };
        let vec = slice.to_vec();
        unsafe { ffi::luau_freebytecode(bytecode) };
        Ok(vec)
    }

    /// Load compiled bytecode onto the stack (as a function).
    pub fn load_bytecode(&self, bytecode: &[u8], chunkname: &str) -> Result<()> {
        let c_chunkname = CString::new(chunkname).map_err(|_| Error::Invalid)?;

        let result = unsafe {
            ffi::luau_loadbytecode(
                self.ptr,
                bytecode.as_ptr() as *const c_char,
                bytecode.len(),
                c_chunkname.as_ptr(),
            )
        };

        self.check_error(result)
    }

    /// Call a function on the stack.
    pub fn pcall(&self, nargs: i32, nresults: i32) -> Result<()> {
        let result = unsafe { ffi::luau_pcall(self.ptr, nargs, nresults) };
        self.check_error(result)
    }

    // Stack operations

    /// Get the number of elements on the stack.
    pub fn get_top(&self) -> i32 {
        unsafe { ffi::luau_gettop(self.ptr) }
    }

    /// Set the stack top to a specific index.
    pub fn set_top(&self, idx: i32) {
        unsafe { ffi::luau_settop(self.ptr, idx) };
    }

    /// Pop n elements from the stack.
    pub fn pop(&self, n: i32) {
        unsafe { ffi::luau_pop(self.ptr, n) };
    }

    /// Push nil onto the stack.
    pub fn push_nil(&self) {
        unsafe { ffi::luau_pushnil(self.ptr) };
    }

    /// Push a boolean onto the stack.
    pub fn push_boolean(&self, b: bool) {
        unsafe { ffi::luau_pushboolean(self.ptr, if b { 1 } else { 0 }) };
    }

    /// Push a number onto the stack.
    pub fn push_number(&self, n: f64) {
        unsafe { ffi::luau_pushnumber(self.ptr, n) };
    }

    /// Push a string onto the stack.
    /// Returns an error if the string contains interior NUL bytes.
    pub fn push_string(&self, s: &str) -> Result<()> {
        // Use pushlstring which handles binary data correctly
        unsafe { ffi::luau_pushlstring(self.ptr, s.as_ptr() as *const c_char, s.len()) };
        Ok(())
    }

    /// Get the type of the value at the given index.
    pub fn get_type(&self, idx: i32) -> Type {
        let t = unsafe { ffi::luau_type(self.ptr, idx) };
        Type::from(t)
    }

    /// Get the type name of the value at the given index.
    pub fn type_name(&self, idx: i32) -> &'static str {
        let t = unsafe { ffi::luau_type(self.ptr, idx) };
        let name = unsafe { ffi::luau_typename(self.ptr, t) };
        if name.is_null() {
            return "unknown";
        }
        unsafe { CStr::from_ptr(name) }
            .to_str()
            .unwrap_or("unknown")
    }

    /// Check if the value at the given index is nil.
    pub fn is_nil(&self, idx: i32) -> bool {
        unsafe { ffi::luau_isnil(self.ptr, idx) != 0 }
    }

    /// Check if the value at the given index is a boolean.
    pub fn is_boolean(&self, idx: i32) -> bool {
        unsafe { ffi::luau_isboolean(self.ptr, idx) != 0 }
    }

    /// Check if the value at the given index is a number.
    pub fn is_number(&self, idx: i32) -> bool {
        unsafe { ffi::luau_isnumber(self.ptr, idx) != 0 }
    }

    /// Check if the value at the given index is a string.
    pub fn is_string(&self, idx: i32) -> bool {
        unsafe { ffi::luau_isstring(self.ptr, idx) != 0 }
    }

    /// Check if the value at the given index is a table.
    pub fn is_table(&self, idx: i32) -> bool {
        unsafe { ffi::luau_istable(self.ptr, idx) != 0 }
    }

    /// Check if the value at the given index is a function.
    pub fn is_function(&self, idx: i32) -> bool {
        unsafe { ffi::luau_isfunction(self.ptr, idx) != 0 }
    }

    /// Get the boolean value at the given index.
    pub fn to_boolean(&self, idx: i32) -> bool {
        unsafe { ffi::luau_toboolean(self.ptr, idx) != 0 }
    }

    /// Get the number value at the given index.
    pub fn to_number(&self, idx: i32) -> f64 {
        unsafe { ffi::luau_tonumber(self.ptr, idx) }
    }

    /// Get the string value at the given index.
    pub fn to_string(&self, idx: i32) -> Option<String> {
        let mut len: usize = 0;
        let s = unsafe { ffi::luau_tolstring(self.ptr, idx, &mut len) };
        if s.is_null() {
            return None;
        }
        let slice = unsafe { std::slice::from_raw_parts(s as *const u8, len) };
        String::from_utf8(slice.to_vec()).ok()
    }

    // Table operations

    /// Create a new table and push it onto the stack.
    pub fn new_table(&self) {
        unsafe { ffi::luau_newtable(self.ptr) };
    }

    /// Create a new table with pre-allocated space.
    pub fn create_table(&self, narr: i32, nrec: i32) {
        unsafe { ffi::luau_createtable(self.ptr, narr, nrec) };
    }

    /// Get a field from a table.
    /// Returns an error if the key contains interior NUL bytes.
    pub fn get_field(&self, idx: i32, key: &str) -> Result<()> {
        let c_key = CString::new(key).map_err(|_| Error::Invalid)?;
        unsafe { ffi::luau_getfield(self.ptr, idx, c_key.as_ptr()) };
        Ok(())
    }

    /// Set a field in a table.
    /// Returns an error if the key contains interior NUL bytes.
    pub fn set_field(&self, idx: i32, key: &str) -> Result<()> {
        let c_key = CString::new(key).map_err(|_| Error::Invalid)?;
        unsafe { ffi::luau_setfield(self.ptr, idx, c_key.as_ptr()) };
        Ok(())
    }

    /// Get a global variable.
    /// Returns an error if the name contains interior NUL bytes.
    pub fn get_global(&self, name: &str) -> Result<()> {
        let c_name = CString::new(name).map_err(|_| Error::Invalid)?;
        unsafe { ffi::luau_getglobal(self.ptr, c_name.as_ptr()) };
        Ok(())
    }

    /// Set a global variable.
    /// Returns an error if the name contains interior NUL bytes.
    pub fn set_global(&self, name: &str) -> Result<()> {
        let c_name = CString::new(name).map_err(|_| Error::Invalid)?;
        unsafe { ffi::luau_setglobal(self.ptr, c_name.as_ptr()) };
        Ok(())
    }

    /// Iterate to the next element in a table.
    pub fn next(&self, idx: i32) -> bool {
        unsafe { ffi::luau_next(self.ptr, idx) != 0 }
    }

    /// Get the length of a value.
    pub fn obj_len(&self, idx: i32) -> usize {
        unsafe { ffi::luau_objlen(self.ptr, idx) }
    }

    // Memory and GC

    /// Get memory usage in bytes.
    pub fn memory_usage(&self) -> usize {
        unsafe { ffi::luau_memoryusage(self.ptr) }
    }

    /// Run garbage collection.
    pub fn gc(&self) {
        unsafe { ffi::luau_gc(self.ptr) };
    }

    /// Check if there is enough stack space.
    pub fn check_stack(&self, size: i32) -> bool {
        unsafe { ffi::luau_checkstack(self.ptr, size) != 0 }
    }

    /// Get the Luau version string.
    pub fn version() -> &'static str {
        let v = unsafe { ffi::luau_version() };
        if v.is_null() {
            return "unknown";
        }
        unsafe { CStr::from_ptr(v) }.to_str().unwrap_or("unknown")
    }

    // Function registration

    /// Register a Rust function as a global Luau function.
    ///
    /// # Example
    ///
    /// ```rust
    /// use giztoy_luau::State;
    ///
    /// let state = State::new().unwrap();
    /// state.register_func("add", |s| {
    ///     let a = s.to_number(1);
    ///     let b = s.to_number(2);
    ///     s.push_number(a + b);
    ///     1
    /// }).unwrap();
    /// state.do_string("result = add(1, 2)").unwrap();
    /// ```
    pub fn register_func<F>(&mut self, name: &str, func: F) -> Result<()>
    where
        F: Fn(&State) -> i32 + Send + Sync + 'static,
    {
        // Ensure callback is set
        if self.callback_set.compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst).is_ok() {
            unsafe {
                ffi::luau_setexternalcallback(self.ptr, Some(rust_external_callback));
            }
        }

        // Generate unique ID
        let id = GLOBAL_FUNC_NEXT_ID.fetch_add(1, Ordering::SeqCst);

        // Register in global map
        {
            let mut guard = GLOBAL_FUNCS.write().map_err(|_| Error::Invalid)?;
            if let Some(map) = guard.as_mut() {
                map.insert(id, RustFuncEntry {
                    func: Box::new(func),
                    state_ptr: self.ptr,
                });
            }
        }

        // Track for cleanup
        self.func_ids.push(id);

        // Register in Luau
        let c_name = CString::new(name).map_err(|_| Error::Invalid)?;
        unsafe {
            ffi::luau_registerexternal(self.ptr, c_name.as_ptr(), id);
        }

        Ok(())
    }

    /// Unregister a function by name.
    pub fn unregister_func(&self, name: &str) -> Result<()> {
        self.push_nil();
        self.set_global(name)
    }

    // Helper methods

    fn check_error(&self, result: i32) -> Result<()> {
        match result {
            0 => Ok(()),
            1 => Err(Error::Compile(self.get_last_error())),
            2 => Err(Error::Runtime(self.get_last_error())),
            3 => Err(Error::Memory),
            _ => Err(Error::Invalid),
        }
    }

    fn get_last_error(&self) -> String {
        let err = unsafe { ffi::luau_geterror(self.ptr) };
        if err.is_null() {
            return String::new();
        }
        unsafe { CStr::from_ptr(err) }
            .to_str()
            .unwrap_or("")
            .to_string()
    }
}

impl Drop for State {
    fn drop(&mut self) {
        // Clean up registered functions
        if !self.func_ids.is_empty() {
            if let Ok(mut guard) = GLOBAL_FUNCS.write() {
                if let Some(map) = guard.as_mut() {
                    for id in &self.func_ids {
                        map.remove(id);
                    }
                }
            }
        }
        
        if !self.ptr.is_null() {
            unsafe { ffi::luau_close(self.ptr) };
            self.ptr = std::ptr::null_mut();
        }
    }
}

// External callback function called from C when Luau invokes a registered Rust function
extern "C" fn rust_external_callback(l: *mut ffi::LuauState, callback_id: u64) -> i32 {
    // Look up the function
    let func_result = {
        let guard = match GLOBAL_FUNCS.read() {
            Ok(g) => g,
            Err(_) => return 0,
        };
        
        match guard.as_ref() {
            Some(map) => map.get(&callback_id).map(|entry| {
                // Create a temporary State wrapper for the callback
                // Note: we don't track func_ids here since this is temporary
                (entry.func.as_ref() as *const dyn Fn(&State) -> i32, entry.state_ptr)
            }),
            None => None,
        }
    };
    
    let (func_ptr, state_ptr) = match func_result {
        Some(x) => x,
        None => return 0,
    };
    
    // Verify the state pointer matches
    if state_ptr != l {
        return 0;
    }
    
    // Create temporary State for callback (without ownership)
    let temp_state = State {
        ptr: l,
        func_ids: Vec::new(),
        callback_set: AtomicBool::new(true),
    };
    
    // Call the function with panic recovery
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        let func = unsafe { &*func_ptr };
        func(&temp_state)
    }));
    
    // Forget the temp state so it doesn't clean up
    std::mem::forget(temp_state);
    
    match result {
        Ok(n) => n,
        Err(_) => {
            // Panic occurred - push error string
            let msg = CString::new("Rust function panicked").unwrap();
            unsafe { ffi::luau_pushlstring(l, msg.as_ptr(), msg.as_bytes().len()) };
            0
        }
    }
}

#[cfg(test)]
mod tests;
