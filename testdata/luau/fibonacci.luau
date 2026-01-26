-- Recursive Fibonacci implementation
-- Tests: recursion, function calls, conditionals

function fib(n)
    if n <= 1 then
        return n
    end
    return fib(n - 1) + fib(n - 2)
end

result = fib(10)
-- Expected: result = 55
