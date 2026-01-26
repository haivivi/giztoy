/**
 * Luau C Wrapper Implementation
 */

#include "luau_wrapper.h"

#include <lua.h>
#include <lualib.h>
#include <luacode.h>

#include <cstdlib>
#include <cstring>
#include <string>

/* Internal state structure */
struct LuauState {
    lua_State* L;
    std::string lastError;
    LuauExternalCallback externalCallback;
    uint64_t currentCallbackId;  // Set during external callback execution
};

/* ==========================================================================
 * State Management
 * ========================================================================== */

LuauState* luau_new(void) {
    LuauState* state = new (std::nothrow) LuauState();
    if (!state) {
        return nullptr;
    }

    state->L = luaL_newstate();
    if (!state->L) {
        delete state;
        return nullptr;
    }

    state->externalCallback = nullptr;
    state->currentCallbackId = 0;

    // Store the LuauState pointer in registry for callbacks
    lua_pushlightuserdata(state->L, state);
    lua_setfield(state->L, LUA_REGISTRYINDEX, "_luau_state");

    return state;
}

void luau_close(LuauState* L) {
    if (L) {
        if (L->L) {
            lua_close(L->L);
        }
        delete L;
    }
}

void luau_openlibs(LuauState* L) {
    if (!L || !L->L) return;

    // Open standard libraries
    luaL_openlibs(L->L);
}

/* ==========================================================================
 * Script Execution
 * ========================================================================== */

LuauError luau_dostring(LuauState* L, const char* source, size_t source_len,
                        const char* chunkname, LuauOptLevel opt_level) {
    if (!L || !L->L || !source) {
        return LUAU_ERR_INVALID;
    }

    L->lastError.clear();

    if (source_len == 0) {
        source_len = strlen(source);
    }

    if (!chunkname) {
        chunkname = "=chunk";
    }

    // Set up compile options
    lua_CompileOptions options = {};
    options.optimizationLevel = static_cast<int>(opt_level);

    // Compile the source
    size_t bytecode_len = 0;
    char* bytecode = luau_compile(source, source_len, &options, &bytecode_len);
    if (!bytecode) {
        L->lastError = "Compilation failed";
        return LUAU_ERR_COMPILE;
    }

    // Load the bytecode
    int load_result = luau_load(L->L, chunkname, bytecode, bytecode_len, 0);
    free(bytecode);

    if (load_result != 0) {
        if (lua_isstring(L->L, -1)) {
            L->lastError = lua_tostring(L->L, -1);
            lua_pop(L->L, 1);
        } else {
            L->lastError = "Load failed";
        }
        return LUAU_ERR_COMPILE;
    }

    // Execute the loaded chunk
    int call_result = lua_pcall(L->L, 0, LUA_MULTRET, 0);
    if (call_result != LUA_OK) {
        if (lua_isstring(L->L, -1)) {
            L->lastError = lua_tostring(L->L, -1);
            lua_pop(L->L, 1);
        } else {
            L->lastError = "Runtime error";
        }
        return LUAU_ERR_RUNTIME;
    }

    return LUAU_OK;
}

LuauError luau_compile(const char* source, size_t source_len,
                       LuauOptLevel opt_level,
                       char** out_bytecode, size_t* out_len) {
    if (!source || !out_bytecode || !out_len) {
        return LUAU_ERR_INVALID;
    }

    lua_CompileOptions options = {};
    options.optimizationLevel = static_cast<int>(opt_level);

    size_t bytecode_len = 0;
    char* bytecode = luau_compile(source, source_len, &options, &bytecode_len);

    if (!bytecode || bytecode_len == 0) {
        *out_bytecode = nullptr;
        *out_len = 0;
        return LUAU_ERR_COMPILE;
    }

    // Check for compilation error (bytecode starts with error marker)
    if (bytecode_len > 0 && bytecode[0] == 0) {
        // Error message is in the bytecode
        free(bytecode);
        *out_bytecode = nullptr;
        *out_len = 0;
        return LUAU_ERR_COMPILE;
    }

    *out_bytecode = bytecode;
    *out_len = bytecode_len;
    return LUAU_OK;
}

LuauError luau_loadbytecode(LuauState* L, const char* bytecode,
                            size_t bytecode_len, const char* chunkname) {
    if (!L || !L->L || !bytecode) {
        return LUAU_ERR_INVALID;
    }

    L->lastError.clear();

    if (!chunkname) {
        chunkname = "=chunk";
    }

    int result = luau_load(L->L, chunkname, bytecode, bytecode_len, 0);
    if (result != 0) {
        if (lua_isstring(L->L, -1)) {
            L->lastError = lua_tostring(L->L, -1);
            lua_pop(L->L, 1);
        } else {
            L->lastError = "Load failed";
        }
        return LUAU_ERR_COMPILE;
    }

    return LUAU_OK;
}

LuauError luau_pcall(LuauState* L, int nargs, int nresults) {
    if (!L || !L->L) {
        return LUAU_ERR_INVALID;
    }

    L->lastError.clear();

    int result = lua_pcall(L->L, nargs, nresults, 0);
    if (result != LUA_OK) {
        if (lua_isstring(L->L, -1)) {
            L->lastError = lua_tostring(L->L, -1);
            lua_pop(L->L, 1);
        } else {
            L->lastError = "Runtime error";
        }
        return LUAU_ERR_RUNTIME;
    }

    return LUAU_OK;
}

/* ==========================================================================
 * Stack Operations
 * ========================================================================== */

int luau_gettop(LuauState* L) {
    if (!L || !L->L) return 0;
    return lua_gettop(L->L);
}

void luau_settop(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_settop(L->L, idx);
}

void luau_pop(LuauState* L, int n) {
    if (!L || !L->L) return;
    lua_pop(L->L, n);
}

void luau_pushvalue(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_pushvalue(L->L, idx);
}

void luau_remove(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_remove(L->L, idx);
}

void luau_insert(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_insert(L->L, idx);
}

/* ==========================================================================
 * Type Checking
 * ========================================================================== */

LuauType luau_type(LuauState* L, int idx) {
    if (!L || !L->L) return LUAU_TYPE_NIL;

    int t = lua_type(L->L, idx);
    switch (t) {
        case LUA_TNIL: return LUAU_TYPE_NIL;
        case LUA_TBOOLEAN: return LUAU_TYPE_BOOLEAN;
        case LUA_TNUMBER: return LUAU_TYPE_NUMBER;
        case LUA_TSTRING: return LUAU_TYPE_STRING;
        case LUA_TTABLE: return LUAU_TYPE_TABLE;
        case LUA_TFUNCTION: return LUAU_TYPE_FUNCTION;
        case LUA_TUSERDATA:
        case LUA_TLIGHTUSERDATA: return LUAU_TYPE_USERDATA;
        case LUA_TTHREAD: return LUAU_TYPE_THREAD;
        case LUA_TBUFFER: return LUAU_TYPE_BUFFER;
        case LUA_TVECTOR: return LUAU_TYPE_VECTOR;
        default: return LUAU_TYPE_NIL;
    }
}

const char* luau_typename(LuauState* L, LuauType t) {
    if (!L || !L->L) return "unknown";

    int lua_type_val;
    switch (t) {
        case LUAU_TYPE_NIL: lua_type_val = LUA_TNIL; break;
        case LUAU_TYPE_BOOLEAN: lua_type_val = LUA_TBOOLEAN; break;
        case LUAU_TYPE_NUMBER: lua_type_val = LUA_TNUMBER; break;
        case LUAU_TYPE_STRING: lua_type_val = LUA_TSTRING; break;
        case LUAU_TYPE_TABLE: lua_type_val = LUA_TTABLE; break;
        case LUAU_TYPE_FUNCTION: lua_type_val = LUA_TFUNCTION; break;
        case LUAU_TYPE_USERDATA: lua_type_val = LUA_TUSERDATA; break;
        case LUAU_TYPE_THREAD: lua_type_val = LUA_TTHREAD; break;
        case LUAU_TYPE_BUFFER: lua_type_val = LUA_TBUFFER; break;
        case LUAU_TYPE_VECTOR: lua_type_val = LUA_TVECTOR; break;
        default: return "unknown";
    }
    return lua_typename(L->L, lua_type_val);
}

int luau_isnil(LuauState* L, int idx) {
    if (!L || !L->L) return 1;
    return lua_isnil(L->L, idx);
}

int luau_isboolean(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_isboolean(L->L, idx);
}

int luau_isnumber(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_isnumber(L->L, idx);
}

int luau_isstring(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_isstring(L->L, idx);
}

int luau_istable(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_istable(L->L, idx);
}

int luau_isfunction(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_isfunction(L->L, idx);
}

/* ==========================================================================
 * Push Values
 * ========================================================================== */

void luau_pushnil(LuauState* L) {
    if (!L || !L->L) return;
    lua_pushnil(L->L);
}

void luau_pushboolean(LuauState* L, int b) {
    if (!L || !L->L) return;
    lua_pushboolean(L->L, b);
}

void luau_pushnumber(LuauState* L, double n) {
    if (!L || !L->L) return;
    lua_pushnumber(L->L, n);
}

void luau_pushinteger(LuauState* L, int64_t n) {
    if (!L || !L->L) return;
    // Use lua_Integer to avoid truncation on platforms where it's larger than int
    lua_pushinteger(L->L, static_cast<lua_Integer>(n));
}

void luau_pushstring(LuauState* L, const char* s) {
    if (!L || !L->L) return;
    if (s) {
        lua_pushstring(L->L, s);
    } else {
        lua_pushnil(L->L);
    }
}

void luau_pushlstring(LuauState* L, const char* s, size_t len) {
    if (!L || !L->L) return;
    if (s) {
        lua_pushlstring(L->L, s, len);
    } else {
        lua_pushnil(L->L);
    }
}

/* C function wrapper to convert between calling conventions */
struct CFuncData {
    LuauCFunction func;
};

static int cfunc_wrapper(lua_State* L) {
    // Get the wrapped LuauState from registry
    lua_getfield(L, LUA_REGISTRYINDEX, "_luau_state");
    LuauState* state = static_cast<LuauState*>(lua_touserdata(L, -1));
    lua_pop(L, 1);

    // Get the original function from upvalue
    LuauCFunction func = reinterpret_cast<LuauCFunction>(
        lua_tolightuserdata(L, lua_upvalueindex(1)));

    if (state && func) {
        return func(state);
    }
    return 0;
}

void luau_pushcfunction(LuauState* L, LuauCFunction fn, const char* debugname) {
    if (!L || !L->L || !fn) return;

    // Push the function pointer as light userdata (upvalue)
    lua_pushlightuserdata(L->L, reinterpret_cast<void*>(fn));
    lua_pushcclosure(L->L, cfunc_wrapper, debugname, 1);
}

/* ==========================================================================
 * Read Values
 * ========================================================================== */

int luau_toboolean(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_toboolean(L->L, idx);
}

double luau_tonumber(LuauState* L, int idx) {
    if (!L || !L->L) return 0.0;
    return lua_tonumber(L->L, idx);
}

int64_t luau_tointeger(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return static_cast<int64_t>(lua_tointeger(L->L, idx));
}

const char* luau_tostring(LuauState* L, int idx) {
    if (!L || !L->L) return nullptr;
    return lua_tostring(L->L, idx);
}

size_t luau_strlen(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_objlen(L->L, idx);
}

const char* luau_tolstring(LuauState* L, int idx, size_t* len) {
    if (!L || !L->L) {
        if (len) *len = 0;
        return nullptr;
    }
    return lua_tolstring(L->L, idx, len);
}

/* ==========================================================================
 * Table Operations
 * ========================================================================== */

void luau_newtable(LuauState* L) {
    if (!L || !L->L) return;
    lua_newtable(L->L);
}

void luau_createtable(LuauState* L, int narr, int nrec) {
    if (!L || !L->L) return;
    lua_createtable(L->L, narr, nrec);
}

void luau_getfield(LuauState* L, int idx, const char* key) {
    if (!L || !L->L || !key) return;
    lua_getfield(L->L, idx, key);
}

void luau_setfield(LuauState* L, int idx, const char* key) {
    if (!L || !L->L || !key) return;
    lua_setfield(L->L, idx, key);
}

void luau_gettable(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_gettable(L->L, idx);
}

void luau_settable(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_settable(L->L, idx);
}

void luau_rawget(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_rawget(L->L, idx);
}

void luau_rawset(LuauState* L, int idx) {
    if (!L || !L->L) return;
    lua_rawset(L->L, idx);
}

void luau_rawgeti(LuauState* L, int idx, int n) {
    if (!L || !L->L) return;
    lua_rawgeti(L->L, idx, n);
}

void luau_rawseti(LuauState* L, int idx, int n) {
    if (!L || !L->L) return;
    lua_rawseti(L->L, idx, n);
}

size_t luau_objlen(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_objlen(L->L, idx);
}

int luau_next(LuauState* L, int idx) {
    if (!L || !L->L) return 0;
    return lua_next(L->L, idx);
}

/* ==========================================================================
 * Global Table
 * ========================================================================== */

void luau_getglobal(LuauState* L, const char* name) {
    if (!L || !L->L || !name) return;
    lua_getglobal(L->L, name);
}

void luau_setglobal(LuauState* L, const char* name) {
    if (!L || !L->L || !name) return;
    lua_setglobal(L->L, name);
}

/* ==========================================================================
 * Function Registration
 * ========================================================================== */

void luau_register(LuauState* L, const LuauReg* funcs) {
    if (!L || !L->L || !funcs) return;

    for (const LuauReg* f = funcs; f->name != nullptr; f++) {
        luau_pushcfunction(L, f->func, f->name);
        lua_setglobal(L->L, f->name);
    }
}

void luau_setfuncs(LuauState* L, const LuauReg* funcs) {
    if (!L || !L->L || !funcs) return;

    for (const LuauReg* f = funcs; f->name != nullptr; f++) {
        luau_pushcfunction(L, f->func, f->name);
        lua_setfield(L->L, -2, f->name);
    }
}

/* ==========================================================================
 * External Callback Support (Go/Rust integration)
 * ========================================================================== */

void luau_setexternalcallback(LuauState* L, LuauExternalCallback callback) {
    if (!L) return;
    L->externalCallback = callback;
}

/* Helper: get LuauState from lua_State registry */
static LuauState* get_luau_state(lua_State* L) {
    lua_getfield(L, LUA_REGISTRYINDEX, "_luau_state");
    LuauState* state = static_cast<LuauState*>(lua_touserdata(L, -1));
    lua_pop(L, 1);
    return state;
}

/* External function wrapper - called when Luau invokes a registered external function */
static int external_func_wrapper(lua_State* L) {
    LuauState* state = get_luau_state(L);
    if (!state || !state->externalCallback) {
        return 0;
    }

    // Get callback_id from upvalues (stored as two 32-bit integers for portability)
    uint32_t id_low = static_cast<uint32_t>(lua_tointeger(L, lua_upvalueindex(1)));
    uint32_t id_high = static_cast<uint32_t>(lua_tointeger(L, lua_upvalueindex(2)));
    uint64_t callback_id = (static_cast<uint64_t>(id_high) << 32) | id_low;

    // Store current callback ID for luau_getcallbackid
    state->currentCallbackId = callback_id;

    // Call the external callback
    int result = state->externalCallback(state, callback_id);

    state->currentCallbackId = 0;
    return result;
}

void luau_pushexternalfunc(LuauState* L, uint64_t callback_id, const char* debugname) {
    if (!L || !L->L) return;

    // Push callback_id as two integers (for 32-bit compatibility)
    uint32_t id_low = static_cast<uint32_t>(callback_id & 0xFFFFFFFF);
    uint32_t id_high = static_cast<uint32_t>(callback_id >> 32);
    
    lua_pushinteger(L->L, static_cast<int>(id_low));
    lua_pushinteger(L->L, static_cast<int>(id_high));
    lua_pushcclosure(L->L, external_func_wrapper, debugname, 2);
}

void luau_registerexternal(LuauState* L, const char* name, uint64_t callback_id) {
    if (!L || !L->L || !name) return;

    luau_pushexternalfunc(L, callback_id, name);
    lua_setglobal(L->L, name);
}

uint64_t luau_getcallbackid(LuauState* L) {
    if (!L) return 0;
    return L->currentCallbackId;
}

/* ==========================================================================
 * Error Handling
 * ========================================================================== */

const char* luau_geterror(LuauState* L) {
    if (!L || L->lastError.empty()) {
        return nullptr;
    }
    return L->lastError.c_str();
}

void luau_clearerror(LuauState* L) {
    if (L) {
        L->lastError.clear();
    }
}

void luau_error(LuauState* L, const char* msg) {
    if (!L || !L->L) return;
    if (msg) {
        L->lastError = msg;
        lua_pushstring(L->L, msg);
    }
    lua_error(L->L);
}

/* ==========================================================================
 * Memory Management
 * ========================================================================== */

size_t luau_memoryusage(LuauState* L) {
    if (!L || !L->L) return 0;
    // Lua reports memory in KB, convert to bytes
    return static_cast<size_t>(lua_gc(L->L, LUA_GCCOUNT, 0)) * 1024 +
           static_cast<size_t>(lua_gc(L->L, LUA_GCCOUNTB, 0));
}

void luau_gc(LuauState* L) {
    if (!L || !L->L) return;
    lua_gc(L->L, LUA_GCCOLLECT, 0);
}

void luau_freebytecode(char* bytecode) {
    free(bytecode);
}

/* ==========================================================================
 * Debug/Utility
 * ========================================================================== */

const char* luau_version(void) {
    return "Luau 0.706";
}

char* luau_dumpstack(LuauState* L) {
    if (!L || !L->L) {
        char* empty = static_cast<char*>(malloc(1));
        if (empty) empty[0] = '\0';
        return empty;
    }

    std::string result;
    int top = lua_gettop(L->L);
    result += "Stack size: " + std::to_string(top) + "\n";

    for (int i = 1; i <= top; i++) {
        result += "[" + std::to_string(i) + "] ";
        int t = lua_type(L->L, i);
        switch (t) {
            case LUA_TSTRING:
                result += "string: \"" + std::string(lua_tostring(L->L, i)) + "\"";
                break;
            case LUA_TBOOLEAN:
                result += lua_toboolean(L->L, i) ? "boolean: true" : "boolean: false";
                break;
            case LUA_TNUMBER:
                result += "number: " + std::to_string(lua_tonumber(L->L, i));
                break;
            case LUA_TNIL:
                result += "nil";
                break;
            default:
                result += std::string(lua_typename(L->L, t));
                break;
        }
        result += "\n";
    }

    char* cstr = static_cast<char*>(malloc(result.length() + 1));
    if (cstr) {
        memcpy(cstr, result.c_str(), result.length() + 1);
    }
    return cstr;
}

int luau_checkstack(LuauState* L, int extra) {
    if (!L || !L->L) return 0;
    return lua_checkstack(L->L, extra);
}
