--!strict
-- PAL SDK tests

local haivivi = require("haivivi")

-- Test configuration (from environment variables)
local AUTH_URL = __builtin.env("HAIVIVI_AUTH_URL") or "https://api.stage.haivivi.cn/auth/v1"
local PAL_URL = __builtin.env("HAIVIVI_PAL_URL") or "https://api.stage.haivivi.cn/pal/v1"
local KEY = __builtin.env("HAIVIVI_KEY") or error("HAIVIVI_KEY environment variable is required")

local function test_pal_characters_list()
    __builtin.log("=== Test: PAL Characters List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local pal_client = haivivi.pal.new_client(PAL_URL, auth_client)
    
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
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local pal_client = haivivi.pal.new_client(PAL_URL, auth_client)
    
    local voices, err = pal_client.voices:list()
    
    if err then
        __builtin.log("FAIL: voices list error:", err)
        return false
    end
    
    if not voices then
        __builtin.log("FAIL: voices is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #voices, "voices")
    
    if #voices > 0 then
        local first = voices[1]
        __builtin.log("  First voice:", first.name or first.id)
    end
    
    return true
end

local function test_pal_virtual_devices_list()
    __builtin.log("=== Test: PAL Virtual Devices List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local pal_client = haivivi.pal.new_client(PAL_URL, auth_client)
    
    local devices, err = pal_client.virtual_devices:list()
    
    if err then
        __builtin.log("FAIL: virtual_devices list error:", err)
        return false
    end
    
    if not devices then
        __builtin.log("FAIL: devices is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #devices, "virtual devices")
    
    if #devices > 0 then
        local first = devices[1]
        __builtin.log("  First device:", first.name or first.id)
    end
    
    return true
end

local function test_pal_chat_topics_list()
    __builtin.log("=== Test: PAL Chat Topics List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local pal_client = haivivi.pal.new_client(PAL_URL, auth_client)
    
    local topics, err = pal_client.chat_topics:list()
    
    if err then
        __builtin.log("FAIL: chat_topics list error:", err)
        return false
    end
    
    if not topics then
        __builtin.log("FAIL: topics is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #topics, "chat topics")
    
    return true
end

local function test_pal_plans_list()
    __builtin.log("=== Test: PAL Plans List ===")
    
    local auth_client = haivivi.auth.new_client(AUTH_URL, KEY)
    local _, auth_err = auth_client:refresh_token()
    if auth_err then
        __builtin.log("FAIL: auth error:", auth_err)
        return false
    end
    
    local pal_client = haivivi.pal.new_client(PAL_URL, auth_client)
    
    local plans, err = pal_client.plans:list()
    
    if err then
        __builtin.log("FAIL: plans list error:", err)
        return false
    end
    
    if not plans then
        __builtin.log("FAIL: plans is nil")
        return false
    end
    
    __builtin.log("PASS: Got", #plans, "plans")
    
    return true
end

-- Run all tests
local function run_tests()
    __builtin.log("Starting PAL SDK tests...")
    __builtin.log("")
    
    local passed = 0
    local failed = 0
    
    if test_pal_characters_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_pal_voices_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_pal_virtual_devices_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_pal_chat_topics_list() then
        passed = passed + 1
    else
        failed = failed + 1
    end
    __builtin.log("")
    
    if test_pal_plans_list() then
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
