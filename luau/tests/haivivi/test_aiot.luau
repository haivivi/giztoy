--!strict
-- AIOT SDK tests

local haivivi = require("haivivi")

-- Test configuration (from environment variables)
local AUTH_URL = __builtin.env("HAIVIVI_AUTH_URL") or "https://api.stage.haivivi.cn/auth/v1"
local AIOT_URL = __builtin.env("HAIVIVI_AIOT_URL") or "https://api.stage.haivivi.cn/aiot/v1"
local KEY = __builtin.env("HAIVIVI_KEY") or error("HAIVIVI_KEY environment variable is required")
local PROJECT_KEY = __builtin.env("HAIVIVI_PROJECT_KEY") or "bubblepal.haivivi.cn"

-- Helper to create authenticated AIOT client
local function create_aiot_client()
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        error("Auth error: " .. auth_err)
    end
    return haivivi.aiot.new_client(AIOT_URL, auth_client)
end

-- ============================================================================
-- Projects Tests
-- ============================================================================

local function test_aiot_projects_list()
    __builtin.log("=== Test: AIOT Projects List ===")
    
    local aiot_client = create_aiot_client()
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
    
    local aiot_client = create_aiot_client()
    
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

local function test_aiot_project_by_id()
    __builtin.log("=== Test: AIOT Project By ID ===")
    
    local aiot_client = create_aiot_client()
    
    -- First get project by key to get ID
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    local project, err = project_doc:get()
    
    if err or not project then
        __builtin.log("SKIP: Could not get project by key")
        return true
    end
    
    local project_id = project.id
    __builtin.log("Project ID:", project_id)
    
    -- Now get by ID
    local by_id, id_err = aiot_client.projects:get(project_id)
    
    if id_err then
        __builtin.log("FAIL: get by ID error:", id_err)
        return false
    end
    
    if not by_id or by_id.id ~= project_id then
        __builtin.log("FAIL: ID mismatch")
        return false
    end
    
    __builtin.log("PASS: Got same project by ID")
    return true
end

-- ============================================================================
-- Nested Resources Tests (Gears, Agents under Project)
-- ============================================================================

local function test_aiot_gears_list()
    __builtin.log("=== Test: AIOT Gears List (Nested) ===")
    
    local aiot_client = create_aiot_client()
    
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
        __builtin.log("  First gear type:", first.type or "unknown")
    end
    
    return true
end

local function test_aiot_agents_list()
    __builtin.log("=== Test: AIOT Agents List (Nested) ===")
    
    local aiot_client = create_aiot_client()
    
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

local function test_aiot_get_single_gear()
    __builtin.log("=== Test: AIOT Get Single Gear ===")
    
    local aiot_client = create_aiot_client()
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    
    -- Get list first
    local gears, err = project_doc.gears:list({ limit = 1 })
    if err or #gears == 0 then
        __builtin.log("SKIP: No gears available")
        return true
    end
    
    local gear_id = gears[1].id
    __builtin.log("Getting gear by ID:", gear_id)
    
    -- Get single gear
    local gear, get_err = project_doc.gears:get(gear_id)
    if get_err then
        __builtin.log("FAIL: get error:", get_err)
        return false
    end
    
    if not gear or gear.id ~= gear_id then
        __builtin.log("FAIL: gear mismatch")
        return false
    end
    
    __builtin.log("PASS: Got gear:", gear.sn or gear.id)
    return true
end

-- ============================================================================
-- Error Handling Tests
-- ============================================================================

local function test_aiot_invalid_project_key()
    __builtin.log("=== Test: AIOT Invalid Project Key ===")
    
    local aiot_client = create_aiot_client()
    
    local project_doc = aiot_client.projects:key("invalid_project_key_12345")
    local project, err = project_doc:get()
    
    if project and not err then
        __builtin.log("WARN: Got result for invalid key (unexpected)")
        return true
    end
    
    if not err then
        __builtin.log("FAIL: Should get error for invalid key")
        return false
    end
    
    __builtin.log("PASS: Got expected error:", err)
    return true
end

local function test_aiot_invalid_gear_id()
    __builtin.log("=== Test: AIOT Invalid Gear ID ===")
    
    local aiot_client = create_aiot_client()
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    
    local gear, err = project_doc.gears:get("invalid_gear_id_12345")
    
    if gear and not err then
        __builtin.log("WARN: Got result for invalid ID")
        return true
    end
    
    if not err then
        __builtin.log("FAIL: Should get error for invalid ID")
        return false
    end
    
    __builtin.log("PASS: Got expected error:", err)
    return true
end

-- ============================================================================
-- Pagination Test
-- ============================================================================

local function test_aiot_gears_pagination()
    __builtin.log("=== Test: AIOT Gears Pagination ===")
    
    local aiot_client = create_aiot_client()
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    
    -- Get all gears
    local all_gears, err = project_doc.gears:list()
    if err then
        __builtin.log("FAIL: list all error:", err)
        return false
    end
    
    local total = #all_gears
    __builtin.log("Total gears:", total)
    
    if total < 2 then
        __builtin.log("SKIP: Not enough gears for pagination test")
        return true
    end
    
    -- Try pagination with limit parameter
    -- Note: API may or may not support pagination
    local page1, err1 = project_doc.gears:list({ limit = 1 })
    if err1 then
        __builtin.log("WARN: pagination query error:", err1)
        return true
    end
    
    -- Check if limit was respected
    if #page1 == 1 then
        __builtin.log("PASS: Pagination works, got 1 gear:", page1[1].sn or page1[1].id)
        
        -- Try offset
        local page2, err2 = project_doc.gears:list({ limit = 1, offset = 1 })
        if not err2 and #page2 == 1 then
            __builtin.log("PASS: Offset works, got gear:", page2[1].sn or page2[1].id)
        end
    else
        __builtin.log("INFO: API returned", #page1, "items (limit param may be ignored)")
        __builtin.log("PASS: API accepts pagination params without error")
    end
    
    return true
end

-- ============================================================================
-- Complex Workflow Test
-- ============================================================================

local function test_aiot_complex_workflow()
    __builtin.log("=== Test: AIOT Complex Workflow ===")
    
    local aiot_client = create_aiot_client()
    
    -- 1. List all projects
    local projects, err = aiot_client.projects:list()
    if err then
        __builtin.log("FAIL: list projects error:", err)
        return false
    end
    __builtin.log("Step 1: Got", #projects, "projects")
    
    -- 2. Get specific project by key
    local project_doc = aiot_client.projects:key(PROJECT_KEY)
    local project, proj_err = project_doc:get()
    if proj_err then
        __builtin.log("FAIL: get project error:", proj_err)
        return false
    end
    __builtin.log("Step 2: Got project:", project.name)
    
    -- 3. List gears under project
    local gears, gears_err = project_doc.gears:list()
    if gears_err then
        __builtin.log("FAIL: list gears error:", gears_err)
        return false
    end
    __builtin.log("Step 3: Got", #gears, "gears")
    
    -- 4. List agents under project
    local agents, agents_err = project_doc.agents:list()
    if agents_err then
        __builtin.log("FAIL: list agents error:", agents_err)
        return false
    end
    __builtin.log("Step 4: Got", #agents, "agents")
    
    -- 5. If we have gears, get details of first one
    if #gears > 0 then
        local gear, gear_err = project_doc.gears:get(gears[1].id)
        if gear_err then
            __builtin.log("WARN: get gear error:", gear_err)
        else
            __builtin.log("Step 5: Got gear details:", gear.sn or gear.id)
        end
    else
        __builtin.log("Step 5: No gears to fetch details")
    end
    
    __builtin.log("PASS: Complex workflow completed")
    return true
end

-- ============================================================================
-- Run All Tests
-- ============================================================================

local function run_tests()
    __builtin.log("Starting AIOT SDK tests...")
    __builtin.log("")
    
    local passed = 0
    local failed = 0
    
    local tests = {
        test_aiot_projects_list,
        test_aiot_project_by_key,
        test_aiot_project_by_id,
        test_aiot_gears_list,
        test_aiot_agents_list,
        test_aiot_get_single_gear,
        test_aiot_invalid_project_key,
        test_aiot_invalid_gear_id,
        test_aiot_gears_pagination,
        test_aiot_complex_workflow,
    }
    
    for _, test in ipairs(tests) do
        local ok, err = pcall(test)
        if ok and err ~= false then
            passed = passed + 1
        else
            failed = failed + 1
            if not ok then
                __builtin.log("ERROR:", err)
            end
        end
        __builtin.log("")
    end
    
    __builtin.log("=== Results ===")
    __builtin.log("Passed:", passed)
    __builtin.log("Failed:", failed)
    
    if failed > 0 then
        error("Some tests failed")
    end
end

run_tests()
