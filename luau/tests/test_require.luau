--!strict
-- Test require function

__builtin.log("=== Testing Require ===")

-- Test require haivivi
local haivivi = require("haivivi")
__builtin.log("haivivi module:", type(haivivi))

-- Test sub-modules
__builtin.log("haivivi.http:", type(haivivi.http))
__builtin.log("haivivi.auth:", type(haivivi.auth))
__builtin.log("haivivi.pal:", type(haivivi.pal))
__builtin.log("haivivi.aiot:", type(haivivi.aiot))

-- Test http.new
local http_client = haivivi.http.new("https://httpbin.org", {})
__builtin.log("http_client:", type(http_client))
__builtin.log("http_client.get:", type(http_client.get))

__builtin.log("=== Require OK ===")
