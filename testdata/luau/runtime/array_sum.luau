-- Array sum: sums all elements in input array
local args = rt:input()
local sum = 0
for i, v in ipairs(args) do
    sum = sum + v
end
rt:output(sum, nil)
