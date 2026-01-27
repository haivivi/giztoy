-- Coroutine status test
-- Tests: coroutine.status, coroutine.running

-- Test initial status
local co = coroutine.create(function()
    coroutine.yield()
    coroutine.yield()
end)

assert(coroutine.status(co) == "suspended", "New coroutine should be suspended")

-- Test status transitions
coroutine.resume(co)
assert(coroutine.status(co) == "suspended", "After first yield should be suspended")

coroutine.resume(co)
assert(coroutine.status(co) == "suspended", "After second yield should be suspended")

coroutine.resume(co)
assert(coroutine.status(co) == "dead", "After completion should be dead")

-- Test coroutine.running()
-- Note: In Luau, coroutine.running() returns nil when called from main thread
local main = coroutine.running()
-- main can be nil in Luau (unlike standard Lua)

local inside_co = nil
local co2 = coroutine.create(function()
    inside_co = coroutine.running()
end)

coroutine.resume(co2)
assert(inside_co ~= nil, "Inside coroutine should capture running")

-- Test resuming dead coroutine
local ok, err = coroutine.resume(co)
assert(not ok, "Resuming dead coroutine should fail")

print("coroutine_status: PASS")
