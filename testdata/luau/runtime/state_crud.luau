-- State CRUD: tests state_get, state_set, state_delete operations
-- Set some state
rt:state_set("counter", 42)
rt:state_set("name", "test")
rt:state_set("to_delete", "will be deleted")

-- Delete one key
rt:state_delete("to_delete")

-- Read it back
local counter = rt:state_get("counter")
local name = rt:state_get("name")
local deleted = rt:state_get("to_delete")
local missing = rt:state_get("missing")

rt:output({
    counter = counter,
    name = name,
    deleted = deleted,
    missing = missing
}, nil)
