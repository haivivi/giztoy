--!strict
-- JSON encoding/decoding tests

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

local function assert_table_eq(name: string, expected: any, actual: any)
    local expected_json = __builtin.json_encode(expected)
    local actual_json = __builtin.json_encode(actual)
    if expected_json == actual_json then
        __builtin.log("PASS:", name)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "expected:", expected_json, "got:", actual_json)
        failed = failed + 1
    end
end

__builtin.log("=== JSON Encoding Tests ===")

-- Basic types
assert_eq("encode nil", "null", __builtin.json_encode(nil))
assert_eq("encode true", "true", __builtin.json_encode(true))
assert_eq("encode false", "false", __builtin.json_encode(false))
assert_eq("encode integer", "42", __builtin.json_encode(42))
assert_eq("encode float", "3.14", __builtin.json_encode(3.14))
assert_eq("encode string", '"hello"', __builtin.json_encode("hello"))
assert_eq("encode empty string", '""', __builtin.json_encode(""))

-- String escaping
assert_eq("encode string with quotes", '"hello \\"world\\""', __builtin.json_encode('hello "world"'))
assert_eq("encode string with newline", '"line1\\nline2"', __builtin.json_encode("line1\nline2"))
assert_eq("encode string with tab", '"a\\tb"', __builtin.json_encode("a\tb"))
assert_eq("encode string with backslash", '"a\\\\b"', __builtin.json_encode("a\\b"))

-- Arrays (note: empty table {} is ambiguous - may encode as [] or {})
local empty_encoded = __builtin.json_encode({})
if empty_encoded == "[]" or empty_encoded == "{}" then
    __builtin.log("PASS: encode empty table (got:", empty_encoded, ")")
    passed = passed + 1
else
    __builtin.log("FAIL: encode empty table, got:", empty_encoded)
    failed = failed + 1
end

local arr_result = __builtin.json_encode({1, 2, 3})
-- Note: arrays may be encoded as objects with numeric keys, so we check decode
local arr_decoded = __builtin.json_decode(arr_result)
assert_eq("encode array length", 3, #arr_decoded)

-- Objects
local obj = { name = "test", value = 123 }
local obj_str = __builtin.json_encode(obj)
local obj_decoded = __builtin.json_decode(obj_str)
assert_eq("encode object name", "test", obj_decoded.name)
assert_eq("encode object value", 123, obj_decoded.value)

-- Nested objects
local nested = {
    user = {
        name = "Alice",
        age = 30
    },
    tags = {1, 2, 3}
}
local nested_str = __builtin.json_encode(nested)
local nested_decoded = __builtin.json_decode(nested_str)
assert_eq("nested object user.name", "Alice", nested_decoded.user.name)
assert_eq("nested object user.age", 30, nested_decoded.user.age)

__builtin.log("")
__builtin.log("=== JSON Decoding Tests ===")

-- Basic types
assert_eq("decode null", nil, __builtin.json_decode("null"))
assert_eq("decode true", true, __builtin.json_decode("true"))
assert_eq("decode false", false, __builtin.json_decode("false"))
assert_eq("decode integer", 42, __builtin.json_decode("42"))
assert_eq("decode float", 3.14, __builtin.json_decode("3.14"))
assert_eq("decode string", "hello", __builtin.json_decode('"hello"'))

-- String escaping
assert_eq("decode escaped quotes", 'hello "world"', __builtin.json_decode('"hello \\"world\\""'))
assert_eq("decode escaped newline", "line1\nline2", __builtin.json_decode('"line1\\nline2"'))

-- Arrays
local arr = __builtin.json_decode("[1, 2, 3]")
assert_eq("decode array length", 3, #arr)
assert_eq("decode array[1]", 1, arr[1])
assert_eq("decode array[2]", 2, arr[2])
assert_eq("decode array[3]", 3, arr[3])

-- Empty array/object
local empty_arr = __builtin.json_decode("[]")
assert_eq("decode empty array type", "table", type(empty_arr))

local empty_obj = __builtin.json_decode("{}")
assert_eq("decode empty object type", "table", type(empty_obj))

-- Complex nested structure
local complex_json = [[{
    "id": 123,
    "name": "test",
    "active": true,
    "data": null,
    "items": [1, 2, 3],
    "meta": {
        "created": "2024-01-01",
        "tags": ["a", "b"]
    }
}]]
local complex = __builtin.json_decode(complex_json)
assert_eq("complex id", 123, complex.id)
assert_eq("complex name", "test", complex.name)
assert_eq("complex active", true, complex.active)
assert_eq("complex data", nil, complex.data)
assert_eq("complex items[1]", 1, complex.items[1])
assert_eq("complex meta.created", "2024-01-01", complex.meta.created)
assert_eq("complex meta.tags[1]", "a", complex.meta.tags[1])

-- Unicode
local unicode_obj = __builtin.json_decode('{"name": "中文测试"}')
assert_eq("decode unicode", "中文测试", unicode_obj.name)

-- Invalid JSON
local invalid = __builtin.json_decode("not valid json")
assert_eq("decode invalid json", nil, invalid)

__builtin.log("")
__builtin.log("=== JSON Roundtrip Tests ===")

-- Test roundtrip (encode then decode)
local function test_roundtrip(name: string, value: any)
    local encoded = __builtin.json_encode(value)
    local decoded = __builtin.json_decode(encoded)
    local re_encoded = __builtin.json_encode(decoded)
    if encoded == re_encoded then
        __builtin.log("PASS:", name)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "encoded:", encoded, "re-encoded:", re_encoded)
        failed = failed + 1
    end
end

test_roundtrip("roundtrip simple object", {a = 1, b = 2})
test_roundtrip("roundtrip nested", {x = {y = {z = 1}}})
test_roundtrip("roundtrip with nil values", {a = 1, b = nil, c = 3})

__builtin.log("")
__builtin.log("=== Results ===")
__builtin.log("Passed:", passed)
__builtin.log("Failed:", failed)

if failed > 0 then
    error("Some tests failed")
end

__builtin.log("=== JSON Tests OK ===")
