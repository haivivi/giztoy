--!strict
-- Tool Runtime Basic Tests
-- Tests basic methods available in tool runtime (rt:*)

local function log(...)
    rt:log("info", ...)
end

local function test_time()
    log("=== Test: rt:time() ===")
    
    local now = rt:time()
    
    if not now or now <= 0 then
        log("FAIL: time() returned invalid value:", now)
        return false
    end
    
    -- Should be a reasonable timestamp (after 2020)
    if now < 1577836800 then -- 2020-01-01
        log("FAIL: time() too old:", now)
        return false
    end
    
    log("PASS: time() =", now)
    return true
end

local function test_parse_time()
    log("=== Test: rt:parse_time() ===")
    
    -- Test ISO 8601 format
    local ts = rt:parse_time("2024-01-15T10:30:00Z")
    
    if not ts then
        log("FAIL: parse_time returned nil")
        return false
    end
    
    -- Expected: around 1705314600-1705315800 (depending on timezone handling)
    -- Just check it's a reasonable timestamp in Jan 2024
    if ts < 1704067200 or ts > 1706745600 then -- Jan 1 - Feb 1, 2024
        log("FAIL: parse_time returned unexpected value:", ts)
        return false
    end
    
    log("PASS: parse_time() =", ts)
    
    -- Test invalid format
    local invalid = rt:parse_time("not a date")
    if invalid ~= nil then
        log("FAIL: parse_time should return nil for invalid input")
        return false
    end
    
    log("PASS: parse_time() returns nil for invalid input")
    return true
end

local function test_json_encode()
    log("=== Test: rt:json_encode() ===")
    
    local data = {
        name = "test",
        value = 123,
        items = {1, 2, 3},
        nested = { a = 1, b = 2 }
    }
    
    local json, err = rt:json_encode(data)
    
    if err then
        log("FAIL: json_encode error:", err)
        return false
    end
    
    if not json or json == "" then
        log("FAIL: json_encode returned empty string")
        return false
    end
    
    -- Check that it contains expected fields
    if not string.find(json, '"name"') then
        log("FAIL: json missing 'name' field")
        return false
    end
    
    log("PASS: json_encode() =", json)
    return true
end

local function test_json_decode()
    log("=== Test: rt:json_decode() ===")
    
    local json = '{"name":"test","value":123,"items":[1,2,3]}'
    local data, err = rt:json_decode(json)
    
    if err then
        log("FAIL: json_decode error:", err)
        return false
    end
    
    if not data then
        log("FAIL: json_decode returned nil")
        return false
    end
    
    if data.name ~= "test" then
        log("FAIL: name mismatch:", data.name)
        return false
    end
    
    if data.value ~= 123 then
        log("FAIL: value mismatch:", data.value)
        return false
    end
    
    log("PASS: json_decode() name =", data.name, "value =", data.value)
    
    -- Test invalid JSON
    local invalid, invalid_err = rt:json_decode("not json")
    if not invalid_err then
        log("FAIL: json_decode should return error for invalid JSON")
        return false
    end
    
    log("PASS: json_decode() returns error for invalid JSON")
    return true
end

local function test_env()
    log("=== Test: rt:env() ===")
    
    -- PATH should exist on most systems
    local path = rt:env("PATH")
    
    if path and path ~= "" then
        log("PASS: env('PATH') exists")
    else
        log("INFO: env('PATH') is nil or empty (may be expected in sandbox)")
    end
    
    -- Non-existent variable should return nil
    local nonexistent = rt:env("NONEXISTENT_VAR_12345")
    if nonexistent ~= nil then
        log("FAIL: env() should return nil for non-existent variable")
        return false
    end
    
    log("PASS: env() returns nil for non-existent variable")
    return true
end

local function test_log()
    log("=== Test: rt:log() ===")
    
    -- Just test that it doesn't error
    rt:log("info", "Test log message", 123, true)
    
    log("PASS: log() executed without error")
    return true
end

local function test_input_output()
    log("=== Test: rt:input() / rt:output() ===")
    
    -- Input may be nil in standalone test
    local input = rt:input()
    
    log("INFO: input() =", input)
    
    -- Test output
    rt:output({ result = "test" }, nil)
    
    log("PASS: output() executed without error")
    return true
end

local function test_uuid()
    log("=== Test: rt:uuid() ===")
    
    -- Skip if uuid method doesn't exist (Rust runner doesn't have it)
    if rt.uuid == nil then
        log("SKIP: rt:uuid() not available")
        return true
    end
    
    local id1 = rt:uuid()
    
    if not id1 or id1 == "" then
        log("FAIL: uuid() returned empty")
        return false
    end
    
    -- UUID should be 36 characters (8-4-4-4-12 format)
    if #id1 ~= 36 then
        log("FAIL: uuid() wrong length:", #id1)
        return false
    end
    
    -- Should have 4 hyphens
    local _, count = string.gsub(id1, "-", "")
    if count ~= 4 then
        log("FAIL: uuid() wrong format:", id1)
        return false
    end
    
    -- Two calls should return different values
    local id2 = rt:uuid()
    if id1 == id2 then
        log("FAIL: uuid() returned same value twice")
        return false
    end
    
    log("PASS: uuid() =", id1)
    return true
end

-- Run all tests
local function run_tests()
    log("Starting Tool Runtime Basic Tests...")
    log("")
    
    local passed = 0
    local failed = 0
    
    local tests = {
        { name = "test_time", fn = test_time },
        { name = "test_parse_time", fn = test_parse_time },
        { name = "test_json_encode", fn = test_json_encode },
        { name = "test_json_decode", fn = test_json_decode },
        { name = "test_env", fn = test_env },
        { name = "test_log", fn = test_log },
        { name = "test_input_output", fn = test_input_output },
        { name = "test_uuid", fn = test_uuid },
    }
    
    for _, test in ipairs(tests) do
        local ok, err = pcall(test.fn)
        if ok and err ~= false then
            passed = passed + 1
        else
            failed = failed + 1
            log("FAILED:", test.name)
            if not ok then
                log("ERROR:", tostring(err))
            end
        end
        log("")
    end
    
    log("=== Results ===")
    log("Passed:", passed)
    log("Failed:", failed)
    
    if failed > 0 then
        error("Some tests failed")
    end
end

run_tests()
