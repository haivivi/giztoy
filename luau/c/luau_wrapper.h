/**
 * Luau C Wrapper
 *
 * A C-compatible wrapper for the Luau scripting language.
 * This wrapper provides a simplified API for embedding Luau in C/Go applications.
 */

#ifndef LUAU_WRAPPER_H
#define LUAU_WRAPPER_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stddef.h>
#include <stdint.h>

/* Opaque Luau state handle */
typedef struct LuauState LuauState;

/* Error codes */
typedef enum {
    LUAU_OK = 0,
    LUAU_ERR_COMPILE = 1,
    LUAU_ERR_RUNTIME = 2,
    LUAU_ERR_MEMORY = 3,
    LUAU_ERR_INVALID = 4,
} LuauError;

/* Luau value types */
typedef enum {
    LUAU_TYPE_NIL = 0,
    LUAU_TYPE_BOOLEAN = 1,
    LUAU_TYPE_NUMBER = 2,
    LUAU_TYPE_STRING = 3,
    LUAU_TYPE_TABLE = 4,
    LUAU_TYPE_FUNCTION = 5,
    LUAU_TYPE_USERDATA = 6,
    LUAU_TYPE_THREAD = 7,
    LUAU_TYPE_BUFFER = 8,
    LUAU_TYPE_VECTOR = 9,
} LuauType;

/* Compiler optimization level */
typedef enum {
    LUAU_OPT_NONE = 0,
    LUAU_OPT_O1 = 1,
    LUAU_OPT_O2 = 2,
} LuauOptLevel;

/* C function callback type */
typedef int (*LuauCFunction)(LuauState* L);

/**
 * External callback type for Go/Rust integration.
 * Called when a registered function is invoked from Luau.
 *
 * @param L The Luau state
 * @param callback_id The unique ID identifying which function to call
 * @return Number of return values pushed onto the stack
 */
typedef int (*LuauExternalCallback)(LuauState* L, uint64_t callback_id);

/* ==========================================================================
 * State Management
 * ========================================================================== */

/**
 * Create a new Luau state.
 * Returns NULL on failure.
 */
LuauState* luau_new(void);

/**
 * Close and free a Luau state.
 * Safe to call with NULL.
 */
void luau_close(LuauState* L);

/**
 * Open standard libraries (math, string, table, os, io, etc.)
 * Note: This function does not perform any sandboxing or exclude libraries.
 * For sandboxed execution, manually restrict globals after calling this.
 */
void luau_openlibs(LuauState* L);

/* ==========================================================================
 * Script Execution
 * ========================================================================== */

/**
 * Compile and execute a Luau script string.
 *
 * @param L The Luau state
 * @param source The Luau source code
 * @param source_len Length of source (0 = use strlen)
 * @param chunkname Name for error messages (can be NULL)
 * @param opt_level Optimization level
 * @return LUAU_OK on success, error code on failure
 */
LuauError luau_dostring(LuauState* L, const char* source, size_t source_len,
                        const char* chunkname, LuauOptLevel opt_level);

/**
 * Compile Luau source to bytecode.
 *
 * @param source The Luau source code
 * @param source_len Length of source
 * @param opt_level Optimization level
 * @param out_bytecode Output bytecode buffer (caller must free)
 * @param out_len Output bytecode length
 * @return LUAU_OK on success, LUAU_ERR_COMPILE on failure
 */
LuauError luau_compile(const char* source, size_t source_len,
                       LuauOptLevel opt_level,
                       char** out_bytecode, size_t* out_len);

/**
 * Load compiled bytecode.
 *
 * @param L The Luau state
 * @param bytecode The compiled bytecode
 * @param bytecode_len Length of bytecode
 * @param chunkname Name for error messages
 * @return LUAU_OK on success, error code on failure
 */
LuauError luau_loadbytecode(LuauState* L, const char* bytecode,
                            size_t bytecode_len, const char* chunkname);

/**
 * Call a function on the stack.
 *
 * @param L The Luau state
 * @param nargs Number of arguments (on stack above function)
 * @param nresults Number of expected results
 * @return LUAU_OK on success, LUAU_ERR_RUNTIME on failure
 */
LuauError luau_pcall(LuauState* L, int nargs, int nresults);

/* ==========================================================================
 * Stack Operations
 * ========================================================================== */

/**
 * Get the current stack top index.
 */
int luau_gettop(LuauState* L);

/**
 * Set the stack top to a specific index.
 * Negative values are relative to current top.
 */
void luau_settop(LuauState* L, int idx);

/**
 * Pop n values from the stack.
 */
void luau_pop(LuauState* L, int n);

/**
 * Push a copy of a value at the given index onto the stack.
 */
void luau_pushvalue(LuauState* L, int idx);

/**
 * Remove a value at the given index, shifting elements down.
 */
void luau_remove(LuauState* L, int idx);

/**
 * Insert the top value at the given index, shifting elements up.
 */
void luau_insert(LuauState* L, int idx);

/* ==========================================================================
 * Type Checking
 * ========================================================================== */

/**
 * Get the type of a value at the given index.
 */
LuauType luau_type(LuauState* L, int idx);

/**
 * Get the type name as a string.
 */
const char* luau_typename(LuauState* L, LuauType t);

/**
 * Check if a value is nil.
 */
int luau_isnil(LuauState* L, int idx);

/**
 * Check if a value is a boolean.
 */
int luau_isboolean(LuauState* L, int idx);

/**
 * Check if a value is a number.
 */
int luau_isnumber(LuauState* L, int idx);

/**
 * Check if a value is a string.
 */
int luau_isstring(LuauState* L, int idx);

/**
 * Check if a value is a table.
 */
int luau_istable(LuauState* L, int idx);

/**
 * Check if a value is a function.
 */
int luau_isfunction(LuauState* L, int idx);

/* ==========================================================================
 * Push Values
 * ========================================================================== */

/**
 * Push nil onto the stack.
 */
void luau_pushnil(LuauState* L);

/**
 * Push a boolean onto the stack.
 */
void luau_pushboolean(LuauState* L, int b);

/**
 * Push a number onto the stack.
 */
void luau_pushnumber(LuauState* L, double n);

/**
 * Push an integer onto the stack.
 */
void luau_pushinteger(LuauState* L, int64_t n);

/**
 * Push a string onto the stack.
 * The string is copied.
 */
void luau_pushstring(LuauState* L, const char* s);

/**
 * Push a string with explicit length onto the stack.
 */
void luau_pushlstring(LuauState* L, const char* s, size_t len);

/**
 * Push a C function onto the stack.
 */
void luau_pushcfunction(LuauState* L, LuauCFunction fn, const char* debugname);

/* ==========================================================================
 * Read Values
 * ========================================================================== */

/**
 * Convert a value to boolean.
 */
int luau_toboolean(LuauState* L, int idx);

/**
 * Convert a value to number.
 * Returns 0 if not a number.
 */
double luau_tonumber(LuauState* L, int idx);

/**
 * Convert a value to integer.
 * Returns 0 if not a number.
 */
int64_t luau_tointeger(LuauState* L, int idx);

/**
 * Convert a value to string.
 * Returns NULL if not a string.
 * The returned pointer is valid until the value is popped.
 */
const char* luau_tostring(LuauState* L, int idx);

/**
 * Get the length of a string value.
 */
size_t luau_strlen(LuauState* L, int idx);

/**
 * Convert a value to string with explicit length output.
 */
const char* luau_tolstring(LuauState* L, int idx, size_t* len);

/* ==========================================================================
 * Table Operations
 * ========================================================================== */

/**
 * Create a new empty table and push it onto the stack.
 */
void luau_newtable(LuauState* L);

/**
 * Create a new table with pre-allocated space.
 *
 * @param narr Hint for number of array elements
 * @param nrec Hint for number of record elements
 */
void luau_createtable(LuauState* L, int narr, int nrec);

/**
 * Get a field from a table.
 * Pushes the value onto the stack.
 *
 * @param L The Luau state
 * @param idx Index of the table
 * @param key Field name
 */
void luau_getfield(LuauState* L, int idx, const char* key);

/**
 * Set a field in a table.
 * Pops the value from the stack.
 *
 * @param L The Luau state
 * @param idx Index of the table
 * @param key Field name
 */
void luau_setfield(LuauState* L, int idx, const char* key);

/**
 * Get a value from a table using a key on the stack.
 * Replaces the key with the value.
 */
void luau_gettable(LuauState* L, int idx);

/**
 * Set a value in a table using key and value on the stack.
 * Pops both key and value.
 */
void luau_settable(LuauState* L, int idx);

/**
 * Get raw table value (bypasses metamethods).
 */
void luau_rawget(LuauState* L, int idx);

/**
 * Set raw table value (bypasses metamethods).
 */
void luau_rawset(LuauState* L, int idx);

/**
 * Get raw table value by integer index.
 */
void luau_rawgeti(LuauState* L, int idx, int n);

/**
 * Set raw table value by integer index.
 */
void luau_rawseti(LuauState* L, int idx, int n);

/**
 * Get the length of a table/string.
 */
size_t luau_objlen(LuauState* L, int idx);

/**
 * Iterate to the next key-value pair in a table.
 * Key must be on top of stack. Pushes next key and value.
 *
 * @return 1 if there are more elements, 0 if iteration is complete
 */
int luau_next(LuauState* L, int idx);

/* ==========================================================================
 * Global Table
 * ========================================================================== */

/**
 * Get a global variable.
 * Pushes the value onto the stack.
 */
void luau_getglobal(LuauState* L, const char* name);

/**
 * Set a global variable.
 * Pops the value from the stack.
 */
void luau_setglobal(LuauState* L, const char* name);

/* ==========================================================================
 * Function Registration
 * ========================================================================== */

/**
 * Function registration entry.
 */
typedef struct {
    const char* name;
    LuauCFunction func;
} LuauReg;

/**
 * Register multiple C functions as globals.
 * The array must be terminated with {NULL, NULL}.
 */
void luau_register(LuauState* L, const LuauReg* funcs);

/**
 * Register C functions into a table at stack top.
 */
void luau_setfuncs(LuauState* L, const LuauReg* funcs);

/**
 * Set the external callback handler for this state.
 * This callback will be invoked when functions registered with
 * luau_pushexternalfunc are called from Luau.
 *
 * @param L The Luau state
 * @param callback The callback function (NULL to disable)
 */
void luau_setexternalcallback(LuauState* L, LuauExternalCallback callback);

/**
 * Push an external function onto the stack.
 * When called from Luau, this will invoke the external callback
 * with the given callback_id.
 *
 * @param L The Luau state
 * @param callback_id Unique ID to identify this function
 * @param debugname Name for debugging (can be NULL)
 */
void luau_pushexternalfunc(LuauState* L, uint64_t callback_id, const char* debugname);

/**
 * Register an external function as a global.
 *
 * @param L The Luau state
 * @param name Global name for the function
 * @param callback_id Unique ID to identify this function
 */
void luau_registerexternal(LuauState* L, const char* name, uint64_t callback_id);

/**
 * Get the callback ID from an upvalue (for use in external callbacks).
 * Call this from within an external callback to get the callback_id.
 *
 * @param L The Luau state
 * @return The callback ID
 */
uint64_t luau_getcallbackid(LuauState* L);

/* ==========================================================================
 * Error Handling
 * ========================================================================== */

/**
 * Get the last error message.
 * Returns NULL if no error.
 */
const char* luau_geterror(LuauState* L);

/**
 * Clear the last error.
 */
void luau_clearerror(LuauState* L);

/**
 * Raise an error with a message.
 * This function does not return.
 */
void luau_error(LuauState* L, const char* msg);

/* ==========================================================================
 * Memory Management
 * ========================================================================== */

/**
 * Get current memory usage in bytes.
 */
size_t luau_memoryusage(LuauState* L);

/**
 * Run garbage collector.
 */
void luau_gc(LuauState* L);

/**
 * Free bytecode buffer allocated by luau_compile.
 */
void luau_freebytecode(char* bytecode);

/* ==========================================================================
 * Coroutine/Thread Support
 * ========================================================================== */

/**
 * Coroutine status codes
 */
typedef enum {
    LUAU_COSTAT_OK = 0,        /* Running or finished successfully */
    LUAU_COSTAT_YIELD = 1,     /* Yielded */
    LUAU_COSTAT_ERRRUN = 2,    /* Runtime error */
    LUAU_COSTAT_ERRSYNTAX = 3, /* Syntax error */
    LUAU_COSTAT_ERRMEM = 4,    /* Memory allocation error */
    LUAU_COSTAT_ERRERR = 5,    /* Error in error handler */
    LUAU_COSTAT_BREAK = 6,     /* Break requested */
} LuauCoStatus;

/**
 * Create a new coroutine (thread).
 * Pushes the new thread onto the stack of L.
 *
 * @param L The parent Luau state
 * @return The new thread's state
 */
LuauState* luau_newthread(LuauState* L);

/**
 * Close a thread's wrapper without closing the underlying lua_State.
 * The lua_State is owned by the parent and will be garbage collected.
 * This only frees the C++ LuauState wrapper allocated by luau_newthread.
 *
 * @param L The thread state to close (must be created by luau_newthread)
 */
void luau_close_thread(LuauState* L);

/**
 * Resume a coroutine.
 * Arguments should be pushed onto the coroutine's stack before calling.
 *
 * @param L The coroutine state to resume
 * @param from The state that is resuming (can be NULL)
 * @param nargs Number of arguments on the coroutine's stack
 * @return Status code (LUAU_COSTAT_OK or LUAU_COSTAT_YIELD on success)
 */
LuauCoStatus luau_resume(LuauState* L, LuauState* from, int nargs);

/**
 * Yield from a coroutine.
 * This function should be called as: return luau_yield(L, nresults);
 * in a C function that wants to yield.
 *
 * @param L The Luau state
 * @param nresults Number of results on stack to pass back
 * @return This value should be returned from the C function
 */
int luau_yield(LuauState* L, int nresults);

/**
 * Get the status of a coroutine.
 *
 * @param L The coroutine state
 * @return Status code
 */
LuauCoStatus luau_status(LuauState* L);

/**
 * Check if a coroutine is yieldable.
 * A coroutine is yieldable if it's not the main thread and not in a
 * non-yieldable C call.
 *
 * @param L The Luau state
 * @return 1 if yieldable, 0 otherwise
 */
int luau_isyieldable(LuauState* L);

/**
 * Get the main thread from any thread.
 *
 * @param L Any thread state
 * @return The main thread state
 */
LuauState* luau_mainthread(LuauState* L);

/* ==========================================================================
 * Debug/Utility
 * ========================================================================== */

/**
 * Get Luau version string.
 */
const char* luau_version(void);

/**
 * Dump the stack contents for debugging.
 * Returns a newly allocated string (caller must free).
 */
char* luau_dumpstack(LuauState* L);

/**
 * Check if the stack has enough space.
 *
 * @param extra Number of extra slots needed
 * @return 1 if enough space, 0 otherwise
 */
int luau_checkstack(LuauState* L, int extra);

#ifdef __cplusplus
}
#endif

#endif /* LUAU_WRAPPER_H */
