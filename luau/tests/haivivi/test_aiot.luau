--!strict
-- AIOT SDK tests

local haivivi = require("haivivi")

-- Test configuration (from environment variables)
local AUTH_URL = __builtin.env("HAIVIVI_AUTH_URL") or "https://api.stage.haivivi.cn/auth/v1"
local AIOT_URL = __builtin.env("HAIVIVI_AIOT_URL") or "https://api.stage.haivivi.cn/aiot/v1"
local KEY = __builtin.env("HAIVIVI_KEY") or error("HAIVIVI_KEY environment variable is required")
local PROJECT_KEY = __builtin.env("HAIVIVI_PROJECT_KEY") or "bubblepal.haivivi.cn"

local function test_aiot_projects_list()
    __builtin.log("=== Test: AIOT Projects List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local aiot_client = haivivi.aiot.new_client(AIOT_URL, auth_client)
    
    local projects, err = aiot_client.projects:list()
    
    if err then
        __builtin.log("FAIL: projects list error:", err)
        return false
    end
    
    if not projects then
        __builtin.log("FAIL: projects is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #projects, "projects")
    
    if #projects > 0 then
        local first = projects[1]
        __builtin.log("  First project:", first.name or first.key or first.id)
    end
    
    return true
end

local function test_aiot_project_by_key()
    __builtin.log("=== Test: AIOT Project By Key ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local aiot_client = haivivi.aiot.new_client(AIOT_URL, auth_client)
    
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    local project, err = project_doc:get()
    
    if err then
        __builtin.log("FAIL: project get error:", err)
        return false
    end
    
    if not project then
        __builtin.log("FAIL: project is nil")
        return false
    end
    
    __builtin.log("PASS: Got project:", project.name or project.key)
    __builtin.log("  ID:", project.id)
    __builtin.log("  Owner:", project.owner)
    
    return true
end

local function test_aiot_gears_list()
    __builtin.log("=== Test: AIOT Gears List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local aiot_client = haivivi.aiot.new_client(AIOT_URL, auth_client)
    
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    local gears, err = project_doc.gears:list()
    
    if err then
        __builtin.log("FAIL: gears list error:", err)
        return false
    end
    
    if not gears then
        __builtin.log("FAIL: gears is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #gears, "gears")
    
    if #gears > 0 then
        local first = gears[1]
        __builtin.log("  First gear SN:", first.sn or first.id)
    end
    
    return true
end

local function test_aiot_agents_list()
    __builtin.log("=== Test: AIOT Agents List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local aiot_client = haivivi.aiot.new_client(AIOT_URL, auth_client)
    
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    local agents, err = project_doc.agents:list()
    
    if err then
        __builtin.log("FAIL: agents list error:", err)
        return false
    end
    
    if not agents then
        __builtin.log("FAIL: agents is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #agents, "agents")
    
    return true
end

-- Run all tests
local function run_tests()
    __builtin.log("Starting AIOT SDK tests...")
    __builtin.log("")
    
    local passed = 0
    local failed = 0
    
    if test_aiot_projects_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_aiot_project_by_key() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_aiot_gears_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_aiot_agents_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    __builtin.log("=== Results ===")
    __builtin.log("Passed:", passed)
    __builtin.log("Failed:", failed)
    
    if failed > 0 then
        error("Some tests failed")
    end
end

run_tests()
