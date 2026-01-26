-- Script for testing different optimization levels
-- Tests: loops, local variables, arithmetic

local sum = 0
for i = 1, 100 do
    sum = sum + i
end
result = sum
-- Expected: result = 5050 (1+2+3+...+100)
