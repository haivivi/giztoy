--!strict
-- Tool Runtime Require Tests
-- Tests that require works in tool runtime

local function log(...)
    rt:log("info", ...)
end

local function test_require_haivivi()
    log("=== Test: require haivivi module ===")
    
    local haivivi = require("haivivi")
    
    if not haivivi then
        log("FAIL: require('haivivi') returned nil")
        return false
    end
    
    -- Check that haivivi module has expected exports (lowercase)
    if not haivivi.auth then
        log("FAIL: haivivi.auth not found")
        return false
    end
    
    if not haivivi.pal then
        log("FAIL: haivivi.pal not found")
        return false
    end
    
    log("PASS: require('haivivi') works")
    return true
end

local function test_require_submodule()
    log("=== Test: require submodule ===")
    
    local http = require("haivivi/http")
    
    if not http then
        log("FAIL: require('haivivi/http') returned nil")
        return false
    end
    
    -- Check that http module has expected exports
    if not http.new then
        log("FAIL: http.new not found")
        return false
    end
    
    -- Test creating an HTTP client
    local client = http.new("https://example.com")
    if not client then
        log("FAIL: http.new() returned nil")
        return false
    end
    
    if not client.get then
        log("FAIL: client.get not found")
        return false
    end
    
    log("PASS: require('haivivi/http') works")
    return true
end

-- Run all tests
local function run_tests()
    log("Starting Tool Runtime Require Tests...")
    log("")
    
    local passed = 0
    local failed = 0
    
    local tests = {
        { name = "test_require_haivivi", fn = test_require_haivivi },
        { name = "test_require_submodule", fn = test_require_submodule },
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
