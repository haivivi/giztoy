-- Closure and counter pattern
-- Tests: closures, local variables, function returns

function makeCounter()
    local count = 0
    return function()
        count = count + 1
        return count
    end
end

counter = makeCounter()
a = counter()  -- 1
b = counter()  -- 2
c = counter()  -- 3
-- Expected: c = 3
