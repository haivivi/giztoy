-- Table transform: reads a table, modifies it, and outputs
local args = rt:input()
rt:output({ name = args.name, count = args.count + 1 }, nil)
