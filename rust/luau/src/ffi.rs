//! FFI bindings to the Luau C wrapper.

#![allow(dead_code)]

use std::os::raw::{c_char, c_int};

/// Opaque Luau state handle
pub enum LuauState {}

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
}
