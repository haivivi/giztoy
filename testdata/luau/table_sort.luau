-- Table sorting and concatenation
-- Tests: table library functions

t = {3, 1, 4, 1, 5, 9, 2, 6}
table.sort(t)
result = table.concat(t, ",")
-- Expected: result = "1,1,2,3,4,5,6,9"
