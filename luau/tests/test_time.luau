--!strict
-- Tests for __builtin.time and __builtin.parse_time

local passed = 0
local failed = 0

local function assert_true(name: string, value: boolean)
    if value then
        passed += 1
        print("PASS: " .. name)
    else
        failed += 1
        print("FAIL: " .. name .. " expected true, got false")
    end
end

local function assert_close(name: string, actual: number, expected: number, tolerance: number)
    local diff = math.abs(actual - expected)
    if diff <= tolerance then
        passed += 1
        print("PASS: " .. name)
    else
        failed += 1
        print("FAIL: " .. name .. " expected ~" .. tostring(expected) .. " (±" .. tostring(tolerance) .. "), got " .. tostring(actual))
    end
end

local function assert_nil(name: string, value: any)
    if value == nil then
        passed += 1
        print("PASS: " .. name)
    else
        failed += 1
        print("FAIL: " .. name .. " expected nil, got " .. tostring(value))
    end
end

print("=== Testing Time Builtins ===")

-- Test __builtin.time()
print("\n-- __builtin.time() --")

local now = __builtin.time()
assert_true("time returns number", type(now) == "number")
assert_true("time is positive", now > 0)
-- Should be after 2024-01-01 (1704067200)
assert_true("time is recent", now > 1704067200)
-- Should be before 2100-01-01 (4102444800)
assert_true("time is not too far in future", now < 4102444800)

-- Test __builtin.parse_time()
print("\n-- __builtin.parse_time() --")

-- Known timestamps for testing
-- 2024-01-01T00:00:00Z = 1704067200
local ts_2024 = __builtin.parse_time("2024-01-01T00:00:00Z")
assert_close("parse 2024-01-01T00:00:00Z", ts_2024 or 0, 1704067200, 1)

-- 2026-01-27T00:00:00Z (approximate, actual depends on leap years)
local ts_2026 = __builtin.parse_time("2026-01-27T00:00:00Z")
assert_true("parse 2026-01-27 returns number", type(ts_2026) == "number")
assert_true("parse 2026-01-27 is after 2024", (ts_2026 or 0) > 1704067200)

-- Test with milliseconds
local ts_with_ms = __builtin.parse_time("2024-01-01T00:00:00.500Z")
assert_close("parse with milliseconds", ts_with_ms or 0, 1704067200.5, 0.01)

-- Test date only
local ts_date_only = __builtin.parse_time("2024-01-01")
assert_close("parse date only", ts_date_only or 0, 1704067200, 1)

-- Test with timezone offset
local ts_with_tz = __builtin.parse_time("2024-01-01T08:00:00+08:00")
-- 08:00 +08:00 = 00:00 UTC
assert_close("parse with timezone", ts_with_tz or 0, 1704067200, 1)

-- Test invalid inputs
local ts_empty = __builtin.parse_time("")
assert_nil("parse empty string", ts_empty)

local ts_invalid = __builtin.parse_time("not a date")
assert_nil("parse invalid string", ts_invalid)

-- Test time comparison
print("\n-- Time comparison --")
local past = __builtin.parse_time("2020-01-01T00:00:00Z")
local future = __builtin.parse_time("2030-01-01T00:00:00Z")
assert_true("now > past", now > (past or 0))
assert_true("now < future", now < (future or math.huge))

-- Summary
print("\n=== Results ===")
print("Passed: " .. tostring(passed))
print("Failed: " .. tostring(failed))

if failed > 0 then
    error("Some tests failed!")
end
