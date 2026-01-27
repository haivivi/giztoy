--!strict
-- Coroutine tests for Luau runtime

local function assert_eq(name: string, expected: any, actual: any)
    if expected ~= actual then
        __builtin.log("FAIL:", name, "- expected", expected, "got", actual)
        error("Assertion failed: " .. name)
    end
    __builtin.log("PASS:", name)
end

local function assert_true(name: string, value: any)
    if not value then
        __builtin.log("FAIL:", name, "- expected true")
        error("Assertion failed: " .. name)
    end
    __builtin.log("PASS:", name)
end

-- ============================================================================
-- Test 1: Basic coroutine create/resume/yield
-- ============================================================================

local function test_basic_coroutine()
    __builtin.log("=== Test: Basic Coroutine ===")
    
    local values = {}
    
    local co = coroutine.create(function()
        table.insert(values, 1)
        coroutine.yield()
        table.insert(values, 2)
        coroutine.yield()
        table.insert(values, 3)
    end)
    
    assert_eq("initial status", "suspended", coroutine.status(co))
    
    coroutine.resume(co)
    assert_eq("after first resume", 1, #values)
    assert_eq("status after yield", "suspended", coroutine.status(co))
    
    coroutine.resume(co)
    assert_eq("after second resume", 2, #values)
    
    coroutine.resume(co)
    assert_eq("after third resume", 3, #values)
    assert_eq("final status", "dead", coroutine.status(co))
end

-- ============================================================================
-- Test 2: Coroutine with values
-- ============================================================================

local function test_coroutine_values()
    __builtin.log("=== Test: Coroutine Values ===")
    
    local co = coroutine.create(function(a: number, b: number)
        local sum = a + b
        local x = coroutine.yield(sum)
        return x * 2
    end)
    
    -- First resume passes arguments to the function
    local ok1, result1 = coroutine.resume(co, 10, 20)
    assert_true("first resume ok", ok1)
    assert_eq("yield value", 30, result1)
    
    -- Second resume passes value to yield
    local ok2, result2 = coroutine.resume(co, 5)
    assert_true("second resume ok", ok2)
    assert_eq("return value", 10, result2)
end

-- ============================================================================
-- Test 3: Producer-Consumer pattern
-- ============================================================================

local function test_producer_consumer()
    __builtin.log("=== Test: Producer-Consumer ===")
    
    local function producer()
        for i = 1, 5 do
            coroutine.yield(i * 10)
        end
    end
    
    local co = coroutine.create(producer)
    local results = {}
    
    while coroutine.status(co) ~= "dead" do
        local ok, value = coroutine.resume(co)
        if ok and value ~= nil then
            table.insert(results, value)
        end
    end
    
    assert_eq("result count", 5, #results)
    
    local total = 0
    for _, v in ipairs(results) do
        total = total + v
    end
    assert_eq("total", 150, total) -- 10+20+30+40+50
end

-- ============================================================================
-- Test 4: Multiple coroutines
-- ============================================================================

local function test_multiple_coroutines()
    __builtin.log("=== Test: Multiple Coroutines ===")
    
    local log = {}
    
    local co1 = coroutine.create(function()
        table.insert(log, "co1-start")
        coroutine.yield()
        table.insert(log, "co1-end")
    end)
    
    local co2 = coroutine.create(function()
        table.insert(log, "co2-start")
        coroutine.yield()
        table.insert(log, "co2-end")
    end)
    
    -- Interleave execution
    coroutine.resume(co1)  -- co1-start
    coroutine.resume(co2)  -- co2-start
    coroutine.resume(co1)  -- co1-end
    coroutine.resume(co2)  -- co2-end
    
    assert_eq("log count", 4, #log)
    assert_eq("log[1]", "co1-start", log[1])
    assert_eq("log[2]", "co2-start", log[2])
    assert_eq("log[3]", "co1-end", log[3])
    assert_eq("log[4]", "co2-end", log[4])
end

-- ============================================================================
-- Test 5: Coroutine wrap
-- ============================================================================

local function test_coroutine_wrap()
    __builtin.log("=== Test: Coroutine Wrap ===")
    
    local iter = coroutine.wrap(function()
        for i = 1, 3 do
            coroutine.yield(i)
        end
    end)
    
    assert_eq("first call", 1, iter())
    assert_eq("second call", 2, iter())
    assert_eq("third call", 3, iter())
    -- Fourth call returns nil (coroutine dead)
    assert_eq("fourth call", nil, iter())
end

-- ============================================================================
-- Test 6: Coroutine error handling
-- ============================================================================

local function test_coroutine_error()
    __builtin.log("=== Test: Coroutine Error ===")
    
    local co = coroutine.create(function()
        coroutine.yield(1)
        error("intentional error")
    end)
    
    local ok1, val1 = coroutine.resume(co)
    assert_true("first resume ok", ok1)
    assert_eq("first value", 1, val1)
    
    local ok2, err2 = coroutine.resume(co)
    assert_true("second resume failed", not ok2)
    assert_true("got error message", err2 ~= nil)
    assert_eq("status after error", "dead", coroutine.status(co))
    
    __builtin.log("  Error message:", err2)
end

-- ============================================================================
-- Test 7: Coroutine running status
-- ============================================================================

local function test_coroutine_running()
    __builtin.log("=== Test: Coroutine Running ===")
    
    -- Note: In Luau, coroutine.running() returns nil when called from main thread
    local main_co = coroutine.running()
    -- main_co can be nil in Luau (unlike standard Lua)
    __builtin.log("  Main coroutine:", main_co == nil and "nil" or "exists")
    
    local inside_co = nil
    local co = coroutine.create(function()
        inside_co = coroutine.running()
        coroutine.yield()
    end)
    
    coroutine.resume(co)
    
    assert_true("inside coroutine captured", inside_co ~= nil)
    __builtin.log("PASS:", "coroutine.running() works inside coroutine")
end

-- ============================================================================
-- Test 8: Nested coroutine calls (coroutine calling another coroutine)
-- ============================================================================

local function test_nested_coroutines()
    __builtin.log("=== Test: Nested Coroutines ===")
    
    local inner_results = {}
    local outer_results = {}
    
    local inner_co = coroutine.create(function()
        for i = 1, 3 do
            table.insert(inner_results, "inner-" .. i)
            coroutine.yield(i)
        end
    end)
    
    local outer_co = coroutine.create(function()
        table.insert(outer_results, "outer-start")
        
        -- Outer coroutine drives inner coroutine
        while coroutine.status(inner_co) ~= "dead" do
            local ok, val = coroutine.resume(inner_co)
            if ok and val then
                table.insert(outer_results, "got-" .. val)
            end
            coroutine.yield("outer-yield")
        end
        
        table.insert(outer_results, "outer-end")
    end)
    
    -- Drive outer coroutine
    local yields = {}
    while coroutine.status(outer_co) ~= "dead" do
        local ok, val = coroutine.resume(outer_co)
        if ok and val then
            table.insert(yields, val)
        end
    end
    
    assert_eq("inner results count", 3, #inner_results)
    assert_eq("outer results count", 5, #outer_results) -- start + 3 got + end
    -- Note: yields is 4 because outer yields once more after inner finishes 
    -- (before checking inner's dead status at loop start)
    assert_eq("yield count", 4, #yields)
end

-- ============================================================================
-- Run all tests
-- ============================================================================

local function run_tests()
    __builtin.log("Starting Coroutine tests...")
    __builtin.log("")
    
    local tests = {
        test_basic_coroutine,
        test_coroutine_values,
        test_producer_consumer,
        test_multiple_coroutines,
        test_coroutine_wrap,
        test_coroutine_error,
        test_coroutine_running,
        test_nested_coroutines,
    }
    
    local passed = 0
    local failed = 0
    
    for _, test in ipairs(tests) do
        local ok, err = pcall(test)
        if ok then
            passed = passed + 1
        else
            failed = failed + 1
            __builtin.log("ERROR:", err)
        end
        __builtin.log("")
    end
    
    __builtin.log("=== Results ===")
    __builtin.log("Passed:", passed)
    __builtin.log("Failed:", failed)
    
    if failed > 0 then
        error("Some tests failed")
    end
end

run_tests()
