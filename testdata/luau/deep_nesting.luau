-- Deep table nesting
-- Tests: table creation, deeply nested structures

t = {}
current = t
for i = 1, 100 do
    current.nested = {}
    current = current.nested
end
current.value = 42

-- Traverse back
current = t
depth = 0
while current.nested do
    current = current.nested
    depth = depth + 1
end
result = current.value
-- Expected: result = 42, depth = 100
