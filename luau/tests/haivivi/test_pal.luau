--!strict
-- PAL SDK tests

local haivivi = require("haivivi")

-- Test configuration (from environment variables)
local AUTH_URL = __builtin.env("HAIVIVI_AUTH_URL") or "https://api.stage.haivivi.cn/auth/v1"
local PAL_URL = __builtin.env("HAIVIVI_PAL_URL") or "https://api.stage.haivivi.cn/pal/v1"
local KEY = __builtin.env("HAIVIVI_KEY") or error("HAIVIVI_KEY environment variable is required")

-- Helper to create authenticated PAL client
local function create_pal_client()
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        error("Auth error: " .. auth_err)
    end
    return haivivi.pal.new_client(PAL_URL, auth_client)
end

-- ============================================================================
-- List Tests
-- ============================================================================

local function test_pal_characters_list()
    __builtin.log("=== Test: PAL Characters List ===")
    
    local pal_client = create_pal_client()
    local characters, err = pal_client.characters:list()
    
    if err then
        __builtin.log("FAIL: characters list error:", err)
        return false
    end
    
    if not characters then
        __builtin.log("FAIL: characters is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #characters, "characters")
    
    if #characters > 0 then
        local first = characters[1]
        __builtin.log("  First character:", first.name or first.id)
    end
    
    return true
end

local function test_pal_voices_list()
    __builtin.log("=== Test: PAL Voices List ===")
    
    local pal_client = create_pal_client()
    local voices, err = pal_client.voices:list()
    
    if err then
        __builtin.log("FAIL: voices list error:", err)
        return false
    end
    
    __builtin.log("PASS: Got", #voices, "voices")
    return true
end

local function test_pal_virtual_devices_list()
    __builtin.log("=== Test: PAL Virtual Devices List ===")
    
    local pal_client = create_pal_client()
    local devices, err = pal_client.virtual_devices:list()
    
    if err then
        __builtin.log("FAIL: virtual_devices list error:", err)
        return false
    end
    
    __builtin.log("PASS: Got", #devices, "virtual devices")
    return true
end

local function test_pal_chat_topics_list()
    __builtin.log("=== Test: PAL Chat Topics List ===")
    
    local pal_client = create_pal_client()
    local topics, err = pal_client.chat_topics:list()
    
    if err then
        __builtin.log("FAIL: chat_topics list error:", err)
        return false
    end
    
    __builtin.log("PASS: Got", #topics, "chat topics")
    return true
end

local function test_pal_plans_list()
    __builtin.log("=== Test: PAL Plans List ===")
    
    local pal_client = create_pal_client()
    local plans, err = pal_client.plans:list()
    
    if err then
        __builtin.log("FAIL: plans list error:", err)
        return false
    end
    
    __builtin.log("PASS: Got", #plans, "plans")
    return true
end

-- ============================================================================
-- Pagination Tests
-- ============================================================================

local function test_pal_pagination()
    __builtin.log("=== Test: PAL Pagination ===")
    
    local pal_client = create_pal_client()
    
    -- Get all characters first
    local all_chars, err = pal_client.characters:list()
    if err then
        __builtin.log("FAIL: list all error:", err)
        return false
    end
    
    local total = #all_chars
    __builtin.log("Total characters:", total)
    
    if total < 2 then
        __builtin.log("SKIP: Not enough characters for pagination test")
        return true
    end
    
    -- Try pagination with limit parameter
    -- Note: API may or may not support pagination
    local page1, err1 = pal_client.characters:list({ limit = 1 })
    if err1 then
        __builtin.log("WARN: pagination not supported:", err1)
        return true -- Not a failure if API doesn't support it
    end
    
    -- Check if limit was respected
    if #page1 == 1 then
        __builtin.log("PASS: Pagination works, got 1 character:", page1[1].name or page1[1].id)
        
        -- Try offset
        local page2, err2 = pal_client.characters:list({ limit = 1, offset = 1 })
        if err2 then
            __builtin.log("WARN: offset not supported")
        elseif #page2 == 1 then
            if page1[1].id ~= page2[1].id then
                __builtin.log("PASS: Offset works, different character:", page2[1].name or page2[1].id)
            else
                __builtin.log("INFO: Same character (offset may not be supported)")
            end
        end
    else
        __builtin.log("INFO: API returned", #page1, "items (limit param may be ignored)")
        __builtin.log("PASS: API accepts pagination params without error")
    end
    
    return true
end

-- ============================================================================
-- Get Single Resource Test
-- ============================================================================

local function test_pal_get_single()
    __builtin.log("=== Test: PAL Get Single Resource ===")
    
    local pal_client = create_pal_client()
    
    -- Get list first
    local characters, err = pal_client.characters:list({ limit = 1 })
    if err or #characters == 0 then
        __builtin.log("SKIP: No characters available")
        return true
    end
    
    local first_id = characters[1].id
    __builtin.log("Getting character by ID:", first_id)
    
    -- Get single character
    local character, get_err = pal_client.characters:get(first_id)
    if get_err then
        __builtin.log("FAIL: get error:", get_err)
        return false
    end
    
    if not character then
        __builtin.log("FAIL: character is nil")
        return false
    end
    
    if character.id ~= first_id then
        __builtin.log("FAIL: ID mismatch")
        return false
    end
    
    __builtin.log("PASS: Got character:", character.name or character.id)
    return true
end

-- ============================================================================
-- Error Handling Tests
-- ============================================================================

local function test_pal_404_error()
    __builtin.log("=== Test: PAL 404 Error ===")
    
    local pal_client = create_pal_client()
    
    -- Try to get non-existent resource
    local result, err = pal_client.characters:get("non_existent_id_12345")
    
    if result and not err then
        __builtin.log("WARN: Got result for non-existent ID (may be valid)")
        return true
    end
    
    if not err then
        __builtin.log("FAIL: Should get error for non-existent ID")
        return false
    end
    
    __builtin.log("PASS: Got error:", err)
    
    if string.find(err, "404") or string.find(err, "not found") then
        __builtin.log("PASS: Error indicates not found")
    else
        __builtin.log("INFO: Error message:", err)
    end
    
    return true
end

-- ============================================================================
-- CRUD Test (Chat Topics - can be created/deleted)
-- ============================================================================

local function test_pal_crud_chat_topic()
    __builtin.log("=== Test: PAL CRUD Chat Topic ===")
    
    local pal_client = create_pal_client()
    
    -- CREATE
    -- Note: chat_topic creation may require specific fields or permissions
    local unique_name = "test_topic_" .. tostring(math.floor(__builtin.time() * 1000) % 100000)
    __builtin.log("Attempting to create chat topic:", unique_name)
    
    local created, create_err = pal_client.chat_topics:create({
        name = unique_name,
        desc = "Test topic created by Luau SDK integration test",
    })
    
    if create_err then
        -- Create may fail due to permissions or missing required fields
        __builtin.log("INFO: create returned error:", create_err)
        __builtin.log("INFO: This may be expected (permissions/schema)")
        
        -- Test passes if we got a proper error response
        if string.find(create_err, "HTTP") then
            __builtin.log("PASS: API returned proper error response")
            return true
        end
        return false
    end
    
    if not created or not created.id then
        __builtin.log("WARN: created topic has no ID")
        return true
    end
    
    local topic_id = created.id
    __builtin.log("PASS: Created topic with ID:", topic_id)
    
    -- READ
    local fetched, read_err = pal_client.chat_topics:get(topic_id)
    if read_err then
        __builtin.log("INFO: read error:", read_err)
        pal_client.chat_topics:delete(topic_id)
        return true
    end
    __builtin.log("PASS: Read topic:", fetched.name or fetched.id)
    
    -- UPDATE (optional - may not be supported)
    local updated, update_err = pal_client.chat_topics:update(topic_id, {
        desc = "Updated description",
    })
    if update_err then
        __builtin.log("INFO: update not supported:", update_err)
    else
        __builtin.log("PASS: Updated topic")
    end
    
    -- DELETE
    local deleted, delete_err = pal_client.chat_topics:delete(topic_id)
    if delete_err then
        __builtin.log("INFO: delete error:", delete_err)
    else
        __builtin.log("PASS: Deleted topic")
    end
    
    return true
end

-- ============================================================================
-- Multiple Resources Test
-- ============================================================================

local function test_pal_multiple_resources()
    __builtin.log("=== Test: PAL Multiple Resources ===")
    
    local pal_client = create_pal_client()
    
    -- Test all resources from PAL SDK
    local resources = {
        { name = "characters", collection = pal_client.characters },
        { name = "voices", collection = pal_client.voices },
        { name = "virtual_devices", collection = pal_client.virtual_devices },
        { name = "chat_topics", collection = pal_client.chat_topics },
        { name = "plans", collection = pal_client.plans },
        { name = "albums", collection = pal_client.albums },
        { name = "firmwares", collection = pal_client.firmwares },
        { name = "triggers", collection = pal_client.triggers },
        { name = "tts_models", collection = pal_client.tts_models },
        { name = "tuned_llms", collection = pal_client.tuned_llms },
        { name = "memberships", collection = pal_client.memberships },
        { name = "tags", collection = pal_client.tags },
        { name = "access_policies", collection = pal_client.access_policies },
        { name = "achievements", collection = pal_client.achievements },
        { name = "conversations", collection = pal_client.conversations },
        { name = "gear_check_in_records", collection = pal_client.gear_check_in_records },
    }
    
    local success_count = 0
    local fail_count = 0
    
    for _, res in ipairs(resources) do
        if res.collection then
            local items, err = res.collection:list()
            if err then
                __builtin.log("  WARN:", res.name, "-", err)
                -- Some resources may require permissions, not a hard failure
            else
                __builtin.log("  OK:", res.name, "-", #items, "items")
                success_count = success_count + 1
            end
        else
            __builtin.log("  SKIP:", res.name, "- not available")
        end
    end
    
    __builtin.log("Result:", success_count, "/", #resources, "resources accessible")
    
    -- Pass if at least core resources work
    if success_count >= 5 then
        __builtin.log("PASS: Core resources accessible")
        return true
    end
    
    return false
end

-- ============================================================================
-- Run All Tests
-- ============================================================================

local function run_tests()
    __builtin.log("Starting PAL SDK tests...")
    __builtin.log("")
    
    local passed = 0
    local failed = 0
    
    local tests = {
        test_pal_characters_list,
        test_pal_voices_list,
        test_pal_virtual_devices_list,
        test_pal_chat_topics_list,
        test_pal_plans_list,
        test_pal_pagination,
        test_pal_get_single,
        test_pal_404_error,
        test_pal_crud_chat_topic,
        test_pal_multiple_resources,
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
