--!strict
-- Tool Runtime Stream Tests
-- Tests generate and transformer stream methods

local function log(...)
    rt:log("info", ...)
end

local function test_generate_not_configured()
    log("=== Test: rt:generate() not configured ===")
    
    -- Without a generator configured, generate should return an error
    local stream, err = rt:generate("test-model", { system = "test", messages = {} })
    
    if stream ~= nil then
        log("FAIL: generate() should return nil when not configured")
        return false
    end
    
    if not err then
        log("FAIL: generate() should return error when not configured")
        return false
    end
    
    if not string.find(err, "not configured") then
        log("INFO: generate() error:", err)
    end
    
    log("PASS: generate() returns error when not configured")
    return true
end

local function test_transformer_not_configured()
    log("=== Test: rt:transformer() not configured ===")
    
    -- Without a transformer configured, transformer should return an error
    local stream, err = rt:transformer("test-transformer", {})
    
    if stream ~= nil then
        log("FAIL: transformer() should return nil when not configured")
        return false
    end
    
    if not err then
        log("FAIL: transformer() should return error when not configured")
        return false
    end
    
    if not string.find(err, "not") then
        log("INFO: transformer() error:", err)
    end
    
    log("PASS: transformer() returns error when not configured")
    return true
end

local function test_generate_method_exists()
    log("=== Test: rt:generate method exists ===")
    
    if rt.generate == nil then
        log("FAIL: rt.generate is nil")
        return false
    end
    
    if type(rt.generate) ~= "function" then
        log("FAIL: rt.generate is not a function")
        return false
    end
    
    log("PASS: rt.generate method exists")
    return true
end

local function test_transformer_method_exists()
    log("=== Test: rt:transformer method exists ===")
    
    if rt.transformer == nil then
        log("FAIL: rt.transformer is nil")
        return false
    end
    
    if type(rt.transformer) ~= "function" then
        log("FAIL: rt.transformer is not a function")
        return false
    end
    
    log("PASS: rt.transformer method exists")
    return true
end

-- Run all tests
local function run_tests()
    log("Starting Tool Runtime Stream Tests...")
    log("")
    
    local passed = 0
    local failed = 0
    
    local tests = {
        { name = "test_generate_method_exists", fn = test_generate_method_exists },
        { name = "test_transformer_method_exists", fn = test_transformer_method_exists },
        { name = "test_generate_not_configured", fn = test_generate_not_configured },
        { name = "test_transformer_not_configured", fn = test_transformer_not_configured },
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
