--!strict
-- Environment variable tests

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

local function assert_not_nil(name: string, value: any)
    if value ~= nil then
        __builtin.log("PASS:", name, "value:", value)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "expected non-nil value")
        failed = failed + 1
    end
end

__builtin.log("=== Environment Variable Tests ===")

-- Test common environment variables that should exist
-- PATH should exist on most systems
local path = __builtin.env("PATH")
assert_not_nil("PATH exists", path)

-- HOME should exist on Unix-like systems
local home = __builtin.env("HOME")
if home then
    __builtin.log("PASS: HOME exists:", home)
    passed = passed + 1
else
    __builtin.log("INFO: HOME not set (might be Windows)")
end

-- USER or USERNAME should exist
local user = __builtin.env("USER") or __builtin.env("USERNAME")
if user then
    __builtin.log("PASS: USER/USERNAME exists:", user)
    passed = passed + 1
else
    __builtin.log("INFO: USER not set")
end

__builtin.log("")
__builtin.log("=== Non-existent Variables ===")

-- Test non-existent variable
assert_nil("non-existent var", __builtin.env("THIS_VAR_SHOULD_NOT_EXIST_12345"))

-- Test empty string key
local empty_result = __builtin.env("")
assert_nil("empty key", empty_result)

__builtin.log("")
__builtin.log("=== HAIVIVI_* Variables ===")

-- These are the variables used by the SDK tests
local haivivi_key = __builtin.env("HAIVIVI_KEY")
if haivivi_key then
    -- Don't log the actual key for security
    __builtin.log("INFO: HAIVIVI_KEY is set (length:", #haivivi_key, ")")
else
    __builtin.log("INFO: HAIVIVI_KEY not set (expected for unit tests)")
end

local auth_url = __builtin.env("HAIVIVI_AUTH_URL")
if auth_url then
    __builtin.log("INFO: HAIVIVI_AUTH_URL:", auth_url)
else
    __builtin.log("INFO: HAIVIVI_AUTH_URL not set (will use default)")
end

local pal_url = __builtin.env("HAIVIVI_PAL_URL")
if pal_url then
    __builtin.log("INFO: HAIVIVI_PAL_URL:", pal_url)
else
    __builtin.log("INFO: HAIVIVI_PAL_URL not set (will use default)")
end

local aiot_url = __builtin.env("HAIVIVI_AIOT_URL")
if aiot_url then
    __builtin.log("INFO: HAIVIVI_AIOT_URL:", aiot_url)
else
    __builtin.log("INFO: HAIVIVI_AIOT_URL not set (will use default)")
end

__builtin.log("")
__builtin.log("=== Results ===")
__builtin.log("Passed:", passed)
__builtin.log("Failed:", failed)

if failed > 0 then
    error("Some tests failed")
end

__builtin.log("=== Env Tests OK ===")
