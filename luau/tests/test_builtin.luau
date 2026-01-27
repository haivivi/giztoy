--!strict
-- Test builtin functions

__builtin.log("=== Testing Builtins ===")

-- Test log
__builtin.log("Test log:", 1, 2, 3)
__builtin.log("Test log with nil:", nil)
__builtin.log("Test log with bool:", true, false)

-- Test json_encode/decode
local data = { name = "test", value = 123 }
local json_str = __builtin.json_encode(data)
__builtin.log("JSON encode:", json_str)

local decoded = __builtin.json_decode(json_str)
__builtin.log("JSON decode name:", decoded.name)
__builtin.log("JSON decode value:", decoded.value)

-- Test kvs
__builtin.kvs_set("test_key", "test_value")
local val = __builtin.kvs_get("test_key")
__builtin.log("KVS get:", val)

__builtin.kvs_set("test_obj", { a = 1, b = 2 })
local obj = __builtin.kvs_get("test_obj")
__builtin.log("KVS get obj.a:", obj.a)

__builtin.kvs_del("test_key")
local deleted = __builtin.kvs_get("test_key")
__builtin.log("KVS after delete:", deleted)

__builtin.log("=== Builtins OK ===")
