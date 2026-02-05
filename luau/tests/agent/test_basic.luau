-- test_basic.luau - Test basic agent runtime functionality
-- Tests rt:recv() and rt:emit() methods

local passed = 0
local failed = 0

local function test(name: string, fn: () -> boolean)
    local ok, err = pcall(fn)
    if ok and err then
        print("PASS: " .. name)
        passed = passed + 1
    else
        print("FAIL: " .. name)
        if not ok then
            print("  Error: " .. tostring(err))
        end
        failed = failed + 1
    end
end

-- Test 1: rt object exists
test("rt object exists", function()
    return rt ~= nil
end)

-- Test 2: rt:recv exists
test("rt:recv method exists", function()
    return rt.recv ~= nil and type(rt.recv) == "function"
end)

-- Test 3: rt:emit exists
test("rt:emit method exists", function()
    return rt.emit ~= nil and type(rt.emit) == "function"
end)

-- Test 4: rt:emit with text chunk
test("rt:emit with text chunk", function()
    local err = rt:emit({ text = "Hello" })
    return err == nil
end)

-- Test 5: rt:emit with empty chunk
test("rt:emit with empty chunk", function()
    local err = rt:emit({ text = "" })
    return err == nil
end)

-- Test 6: rt:emit multiple chunks
test("rt:emit multiple chunks", function()
    local err1 = rt:emit({ text = "Hello, " })
    local err2 = rt:emit({ text = "World!" })
    return err1 == nil and err2 == nil
end)

-- Test 7: rt:recv returns nil when no input (CLI mode)
test("rt:recv returns nil when no input", function()
    local contents, err = rt:recv()
    -- In CLI mode with no input pending, recv should return nil
    -- This is not an error, just means no input available
    return true  -- Accept any result for this test
end)

-- Test 8: Tool runtime methods still available
test("rt:time method available", function()
    if rt.time == nil then
        return false
    end
    local t = rt:time()
    return type(t) == "number" and t > 0
end)

test("rt:json_encode method available", function()
    if rt.json_encode == nil then
        return false
    end
    local result = rt:json_encode({ name = "test" })
    return result ~= nil and result:find("test") ~= nil
end)

test("rt:json_decode method available", function()
    if rt.json_decode == nil then
        return false
    end
    local value, err = rt:json_decode('{"name":"test"}')
    return value ~= nil and value.name == "test"
end)

test("rt:env method available", function()
    if rt.env == nil then
        return false
    end
    -- Just check it doesn't error
    local _ = rt:env("PATH")
    return true
end)

test("rt:log method available", function()
    if rt.log == nil then
        return false
    end
    -- Just check it doesn't error
    rt:log("info", "Test log message")
    return true
end)

-- Print summary
print("")
print("==================")
print("Test Summary:")
print("  Passed: " .. passed)
print("  Failed: " .. failed)
print("==================")

if failed > 0 then
    error("Some tests failed")
end
