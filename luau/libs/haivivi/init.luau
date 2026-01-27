--!strict
-- Haivivi SDK main entry point.

local haivivi = {}

-- Re-export modules
haivivi.http = require("haivivi/http")
haivivi.auth = require("haivivi/auth")
haivivi.pal = require("haivivi/pal")
haivivi.aiot = require("haivivi/aiot")
haivivi.resource = require("haivivi/resource")

return haivivi
