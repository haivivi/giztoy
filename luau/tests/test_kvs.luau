--!strict
-- KVS (Key-Value Store) tests

local passed = 0
local failed = 0

local function assert_eq(name: string, expected: any, actual: any)
    if expected == actual then
        __builtin.log("PASS:", name)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "expected:", expected, "got:", actual)
        failed = failed + 1
    end
end

local function assert_nil(name: string, value: any)
    if value == nil then
        __builtin.log("PASS:", name)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "expected nil, got:", value)
        failed = failed + 1
    end
end

__builtin.log("=== KVS Basic Operations ===")

-- Test string values
__builtin.kvs_set("test:string", "hello world")
assert_eq("get string", "hello world", __builtin.kvs_get("test:string"))

-- Test number values
__builtin.kvs_set("test:number", 42)
assert_eq("get number", 42, __builtin.kvs_get("test:number"))

__builtin.kvs_set("test:float", 3.14159)
assert_eq("get float", 3.14159, __builtin.kvs_get("test:float"))

-- Test boolean values
__builtin.kvs_set("test:bool:true", true)
assert_eq("get bool true", true, __builtin.kvs_get("test:bool:true"))

__builtin.kvs_set("test:bool:false", false)
assert_eq("get bool false", false, __builtin.kvs_get("test:bool:false"))

-- Test nil/missing keys
assert_nil("get missing key", __builtin.kvs_get("test:nonexistent"))

__builtin.log("")
__builtin.log("=== KVS Object/Table Values ===")

-- Test simple object
local obj = { name = "test", value = 123 }
__builtin.kvs_set("test:obj", obj)
local retrieved_obj = __builtin.kvs_get("test:obj")
assert_eq("get obj.name", "test", retrieved_obj.name)
assert_eq("get obj.value", 123, retrieved_obj.value)

-- Test nested object
local nested = {
    user = {
        id = 1,
        name = "Alice",
        settings = {
            theme = "dark",
            notifications = true
        }
    },
    metadata = {
        version = "1.0",
        created = "2024-01-01"
    }
}
__builtin.kvs_set("test:nested", nested)
local retrieved_nested = __builtin.kvs_get("test:nested")
assert_eq("nested user.id", 1, retrieved_nested.user.id)
assert_eq("nested user.name", "Alice", retrieved_nested.user.name)
assert_eq("nested user.settings.theme", "dark", retrieved_nested.user.settings.theme)
assert_eq("nested metadata.version", "1.0", retrieved_nested.metadata.version)

-- Test array-like table
local arr = {1, 2, 3, 4, 5}
__builtin.kvs_set("test:array", arr)
local retrieved_arr = __builtin.kvs_get("test:array")
assert_eq("array length", 5, #retrieved_arr)
assert_eq("array[1]", 1, retrieved_arr[1])
assert_eq("array[5]", 5, retrieved_arr[5])

__builtin.log("")
__builtin.log("=== KVS Delete Operations ===")

-- Test delete
__builtin.kvs_set("test:delete", "will be deleted")
assert_eq("before delete", "will be deleted", __builtin.kvs_get("test:delete"))

__builtin.kvs_del("test:delete")
assert_nil("after delete", __builtin.kvs_get("test:delete"))

-- Test delete non-existent key (should not error)
__builtin.kvs_del("test:nonexistent")
__builtin.log("PASS: delete non-existent key (no error)")
passed = passed + 1

__builtin.log("")
__builtin.log("=== KVS Overwrite Operations ===")

-- Test overwriting values
__builtin.kvs_set("test:overwrite", "initial")
assert_eq("initial value", "initial", __builtin.kvs_get("test:overwrite"))

__builtin.kvs_set("test:overwrite", "updated")
assert_eq("overwritten value", "updated", __builtin.kvs_get("test:overwrite"))

-- Test overwriting with different type
__builtin.kvs_set("test:type_change", "string")
assert_eq("type before change", "string", type(__builtin.kvs_get("test:type_change")))

__builtin.kvs_set("test:type_change", 123)
assert_eq("type after change", "number", type(__builtin.kvs_get("test:type_change")))

__builtin.log("")
__builtin.log("=== KVS Edge Cases ===")

-- Empty string key (if supported)
__builtin.kvs_set("", "empty key value")
local empty_key_result = __builtin.kvs_get("")
-- This might be nil or the value depending on implementation
__builtin.log("INFO: empty key result:", empty_key_result)

-- Special characters in key
__builtin.kvs_set("test:special:!@#$%", "special chars")
assert_eq("special chars in key", "special chars", __builtin.kvs_get("test:special:!@#$%"))

-- Unicode in key and value
__builtin.kvs_set("test:中文", "Chinese key")
assert_eq("unicode key", "Chinese key", __builtin.kvs_get("test:中文"))

__builtin.kvs_set("test:unicode_value", "值为中文")
assert_eq("unicode value", "值为中文", __builtin.kvs_get("test:unicode_value"))

-- Large value
local large_obj = {}
for i = 1, 100 do
    large_obj["key" .. i] = "value" .. i
end
__builtin.kvs_set("test:large", large_obj)
local retrieved_large = __builtin.kvs_get("test:large")
assert_eq("large obj key1", "value1", retrieved_large.key1)
assert_eq("large obj key100", "value100", retrieved_large.key100)

__builtin.log("")
__builtin.log("=== Cleanup ===")

-- Clean up test keys
local test_keys = {
    "test:string", "test:number", "test:float", 
    "test:bool:true", "test:bool:false",
    "test:obj", "test:nested", "test:array",
    "test:overwrite", "test:type_change",
    "", "test:special:!@#$%", "test:中文", "test:unicode_value",
    "test:large"
}
for _, key in ipairs(test_keys) do
    __builtin.kvs_del(key)
end
__builtin.log("PASS: cleanup complete")
passed = passed + 1

__builtin.log("")
__builtin.log("=== Results ===")
__builtin.log("Passed:", passed)
__builtin.log("Failed:", failed)

if failed > 0 then
    error("Some tests failed")
end

__builtin.log("=== KVS Tests OK ===")
