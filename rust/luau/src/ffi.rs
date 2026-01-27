//! FFI bindings to the Luau C wrapper.

#![allow(dead_code)]

use std::os::raw::{c_char, c_int};

/// Opaque Luau state handle
pub enum LuauState {}

/// Coroutine status codes (matches LuauCoStatus in C wrapper)
#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LuauCoStatus {
    Ok = 0,        // Running or finished successfully
    Yield = 1,     // Yielded
    ErrRun = 2,    // Runtime error
    ErrSyntax = 3, // Syntax error
    ErrMem = 4,    // Memory allocation error
    ErrErr = 5,    // Error in error handler
    Break = 6,     // Break requested
}

#[link(name = "luau_wrapper")]
extern "C" {
    // State management
    pub fn luau_new() -> *mut LuauState;
    pub fn luau_close(L: *mut LuauState);
    pub fn luau_openlibs(L: *mut LuauState);

    // Script execution
    pub fn luau_dostring(
        L: *mut LuauState,
        source: *const c_char,
        source_len: usize,
        chunkname: *const c_char,
        opt_level: c_int,
    ) -> c_int;

    pub fn luau_compile(
        source: *const c_char,
        source_len: usize,
        opt_level: c_int,
        out_bytecode: *mut *mut c_char,
        out_len: *mut usize,
    ) -> c_int;

    pub fn luau_freebytecode(bytecode: *mut c_char);

    pub fn luau_loadbytecode(
        L: *mut LuauState,
        bytecode: *const c_char,
        bytecode_len: usize,
        chunkname: *const c_char,
    ) -> c_int;

    pub fn luau_pcall(L: *mut LuauState, nargs: c_int, nresults: c_int) -> c_int;

    // Stack operations
    pub fn luau_gettop(L: *mut LuauState) -> c_int;
    pub fn luau_settop(L: *mut LuauState, idx: c_int);
    pub fn luau_pop(L: *mut LuauState, n: c_int);
    pub fn luau_pushvalue(L: *mut LuauState, idx: c_int);
    pub fn luau_insert(L: *mut LuauState, idx: c_int);
    pub fn luau_remove(L: *mut LuauState, idx: c_int);
    pub fn luau_checkstack(L: *mut LuauState, size: c_int) -> c_int;

    // Push operations
    pub fn luau_pushnil(L: *mut LuauState);
    pub fn luau_pushboolean(L: *mut LuauState, b: c_int);
    pub fn luau_pushnumber(L: *mut LuauState, n: f64);
    pub fn luau_pushlstring(L: *mut LuauState, s: *const c_char, len: usize);

    // Type checking
    pub fn luau_type(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_typename(L: *mut LuauState, tp: c_int) -> *const c_char;
    pub fn luau_isnil(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_isboolean(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_isnumber(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_isstring(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_istable(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_isfunction(L: *mut LuauState, idx: c_int) -> c_int;

    // Value access
    pub fn luau_toboolean(L: *mut LuauState, idx: c_int) -> c_int;
    pub fn luau_tonumber(L: *mut LuauState, idx: c_int) -> f64;
    pub fn luau_tolstring(L: *mut LuauState, idx: c_int, len: *mut usize) -> *const c_char;

    // Table operations
    pub fn luau_newtable(L: *mut LuauState);
    pub fn luau_createtable(L: *mut LuauState, narr: c_int, nrec: c_int);
    pub fn luau_getfield(L: *mut LuauState, idx: c_int, key: *const c_char);
    pub fn luau_setfield(L: *mut LuauState, idx: c_int, key: *const c_char);
    pub fn luau_gettable(L: *mut LuauState, idx: c_int);
    pub fn luau_settable(L: *mut LuauState, idx: c_int);
    pub fn luau_rawget(L: *mut LuauState, idx: c_int);
    pub fn luau_rawset(L: *mut LuauState, idx: c_int);
    pub fn luau_rawgeti(L: *mut LuauState, idx: c_int, n: c_int);
    pub fn luau_rawseti(L: *mut LuauState, idx: c_int, n: c_int);
    pub fn luau_next(L: *mut LuauState, idx: c_int) -> c_int;

    // Global operations
    pub fn luau_getglobal(L: *mut LuauState, name: *const c_char);
    pub fn luau_setglobal(L: *mut LuauState, name: *const c_char);

    // Misc
    pub fn luau_objlen(L: *mut LuauState, idx: c_int) -> usize;
    pub fn luau_memoryusage(L: *mut LuauState) -> usize;
    pub fn luau_gc(L: *mut LuauState);
    pub fn luau_version() -> *const c_char;

    // Error handling
    pub fn luau_clearerror(L: *mut LuauState);
    pub fn luau_geterror(L: *mut LuauState) -> *const c_char;

    // External callback support
    pub fn luau_setexternalcallback(
        L: *mut LuauState,
        callback: Option<extern "C" fn(*mut LuauState, u64) -> c_int>,
    );
    pub fn luau_pushexternalfunc(L: *mut LuauState, callback_id: u64, debugname: *const c_char);
    pub fn luau_registerexternal(L: *mut LuauState, name: *const c_char, callback_id: u64);
    pub fn luau_getcallbackid(L: *mut LuauState) -> u64;

    // Coroutine/Thread support
    /// Create a new coroutine (thread). Pushes the new thread onto the stack of L.
    /// Returns the new thread's state.
    pub fn luau_newthread(L: *mut LuauState) -> *mut LuauState;

    /// Resume a coroutine. Arguments should be pushed onto the coroutine's stack before calling.
    /// Returns status code (Ok or Yield on success).
    pub fn luau_resume(L: *mut LuauState, from: *mut LuauState, nargs: c_int) -> LuauCoStatus;

    /// Yield from a coroutine. Should be called as: return luau_yield(L, nresults);
    /// Returns the value to be returned from the C function.
    pub fn luau_yield(L: *mut LuauState, nresults: c_int) -> c_int;

    /// Get the status of a coroutine.
    pub fn luau_status(L: *mut LuauState) -> LuauCoStatus;

    /// Check if a coroutine is yieldable.
    /// Returns 1 if yieldable, 0 otherwise.
    pub fn luau_isyieldable(L: *mut LuauState) -> c_int;

    /// Get the main thread from any thread.
    pub fn luau_mainthread(L: *mut LuauState) -> *mut LuauState;
}
