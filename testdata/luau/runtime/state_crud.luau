-- State CRUD: tests state get/set/delete operations

-- Set some state values
rt:state_set("counter", 42)
rt:state_set("name", "test")
rt:state_set("to_delete", "temporary")

-- Delete a key
rt:state_delete("to_delete")

-- Get values back (including deleted one which should be nil)
local counter = rt:state_get("counter")
local name = rt:state_get("name")
local deleted = rt:state_get("to_delete")

rt:output({
    counter = counter,
    name = name,
    deleted = deleted
}, nil)
