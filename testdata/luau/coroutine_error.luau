-- Coroutine error handling test
-- Tests: error propagation, pcall in coroutines

-- Test error in coroutine
local co = coroutine.create(function()
    coroutine.yield("before error")
    error("intentional error")
end)

local ok1, val1 = coroutine.resume(co)
assert(ok1, "First resume should succeed")
assert(val1 == "before error", "Should get yield value")

local ok2, err2 = coroutine.resume(co)
assert(not ok2, "Second resume should fail due to error")
assert(err2 ~= nil, "Should get error message")
assert(coroutine.status(co) == "dead", "Coroutine should be dead after error")

-- Test pcall inside coroutine
local error_caught = false
local co2 = coroutine.create(function()
    local ok, err = pcall(function()
        error("caught error")
    end)
    error_caught = not ok
    coroutine.yield("after pcall")
    return "completed"
end)

local ok3, val3 = coroutine.resume(co2)
assert(ok3, "Resume should succeed even after pcall")
assert(val3 == "after pcall", "Should get yield after pcall")
assert(error_caught, "Error should have been caught by pcall")

local ok4, val4 = coroutine.resume(co2)
assert(ok4, "Final resume should succeed")
assert(val4 == "completed", "Should get return value")

-- Test assert in coroutine
local co3 = coroutine.create(function()
    assert(true, "This should pass")
    coroutine.yield("passed assert")
    assert(false, "This should fail")
end)

local ok5, val5 = coroutine.resume(co3)
assert(ok5 and val5 == "passed assert", "First assert should pass")

local ok6, err6 = coroutine.resume(co3)
assert(not ok6, "Second assert should fail")
assert(string.find(err6, "This should fail"), "Error should contain assert message")

-- Test multiple errors don't crash
for i = 1, 5 do
    local co_err = coroutine.create(function()
        error("error " .. i)
    end)
    local ok, err = coroutine.resume(co_err)
    assert(not ok, "Should fail")
    assert(string.find(err, "error " .. i), "Should contain error number")
end

print("coroutine_error: PASS")
