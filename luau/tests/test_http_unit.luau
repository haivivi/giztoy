--!strict
-- HTTP client unit tests (no network required)

local haivivi = require("haivivi")

local passed = 0
local failed = 0

local function assert_eq(name: string, expected: any, actual: any)
    if expected == actual then
        __builtin.log("PASS:", name)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "expected:", expected, "got:", actual)
        failed = failed + 1
    end
end

local function assert_not_nil(name: string, value: any)
    if value ~= nil then
        __builtin.log("PASS:", name)
        passed = passed + 1
    else
        __builtin.log("FAIL:", name, "expected non-nil value")
        failed = failed + 1
    end
end

__builtin.log("=== HTTP Client Creation Tests ===")

-- Test basic client creation
local client = haivivi.http.new("https://api.example.com", {})
assert_not_nil("client created", client)
assert_eq("client base_url", "https://api.example.com", client.base_url)

-- Test client with custom headers
local client_with_headers = haivivi.http.new("https://api.example.com", {
    headers = {
        ["X-Custom-Header"] = "custom-value",
        ["Accept"] = "application/json"
    }
})
assert_not_nil("client with headers created", client_with_headers)
assert_eq("custom header", "custom-value", client_with_headers.default_headers["X-Custom-Header"])
assert_eq("accept header", "application/json", client_with_headers.default_headers["Accept"])

-- Test client with auth function
local auth_called = false
local client_with_auth = haivivi.http.new("https://api.example.com", {
    auth = function()
        auth_called = true
        return "test-token"
    end
})
assert_not_nil("client with auth created", client_with_auth)
assert_not_nil("auth_fn set", client_with_auth.auth_fn)

__builtin.log("")
__builtin.log("=== HTTP Client Methods Exist ===")

-- Test all HTTP methods exist
assert_eq("get method type", "function", type(client.get))
assert_eq("post method type", "function", type(client.post))
assert_eq("put method type", "function", type(client.put))
assert_eq("patch method type", "function", type(client.patch))
assert_eq("delete method type", "function", type(client.delete))
assert_eq("request method type", "function", type(client.request))

__builtin.log("")
__builtin.log("=== Resource Collection Tests ===")

-- Test resource collection creation
local resource = require("haivivi/resource")

local mock_http = haivivi.http.new("https://api.example.com", {})
local users = resource.new_collection(mock_http, "users")

assert_not_nil("collection created", users)
assert_eq("collection path", "users", users.path)
assert_eq("collection http_client", mock_http, users.http_client)

-- Test collection methods exist
assert_eq("list method type", "function", type(users.list))
assert_eq("get method type", "function", type(users.get))
assert_eq("create method type", "function", type(users.create))
assert_eq("update method type", "function", type(users.update))
assert_eq("delete method type", "function", type(users.delete))
assert_eq("post_verb method type", "function", type(users.post_verb))
assert_eq("get_verb method type", "function", type(users.get_verb))
assert_eq("post_doc_verb method type", "function", type(users.post_doc_verb))

__builtin.log("")
__builtin.log("=== Sub-Collection Tests ===")

-- Test sub-collection creation
local comments = resource.new_sub_collection(mock_http, "posts", "123", "comments")
assert_not_nil("sub-collection created", comments)
assert_eq("sub-collection path", "posts/123/comments", comments.path)

__builtin.log("")
__builtin.log("=== Auth Client Structure Tests ===")

-- Test auth client structure (without making actual requests)
-- We can't actually test refresh_token without network, but we can test the structure
local auth = require("haivivi/auth")

-- Test that new_client function exists and returns proper structure
-- Using a dummy URL and key since we won't make actual requests
local auth_client = auth.new_client("https://dummy.example.com", "dummy-key")
assert_not_nil("auth client created", auth_client)
assert_eq("auth base_url", "https://dummy.example.com", auth_client.base_url)
assert_eq("auth key", "dummy-key", auth_client.key)

-- Check methods exist
assert_eq("refresh_token type", "function", type(auth_client.refresh_token))
assert_eq("get_token type", "function", type(auth_client.get_token))
assert_eq("http_client type", "function", type(auth_client.http_client))

-- Check resource collections exist
assert_not_nil("sessions collection", auth_client.sessions)
assert_not_nil("users collection", auth_client.users)
assert_not_nil("namespaces collection", auth_client.namespaces)

__builtin.log("")
__builtin.log("=== PAL Client Structure Tests ===")

local pal = require("haivivi/pal")

-- Create dummy auth client for PAL
local dummy_auth = auth.new_client("https://dummy.example.com", "dummy-key")
local pal_client = pal.new_client("https://dummy-pal.example.com", dummy_auth)

assert_not_nil("pal client created", pal_client)

-- Check resource collections exist (using actual fields from pal.luau)
assert_not_nil("characters collection", pal_client.characters)
assert_not_nil("voices collection", pal_client.voices)
assert_not_nil("virtual_devices collection", pal_client.virtual_devices)
assert_not_nil("plans collection", pal_client.plans)
assert_not_nil("chat_topics collection", pal_client.chat_topics)
assert_not_nil("albums collection", pal_client.albums)
assert_not_nil("firmwares collection", pal_client.firmwares)
assert_not_nil("triggers collection", pal_client.triggers)
assert_not_nil("tags collection", pal_client.tags)

-- Check methods exist
assert_eq("refresh_token type", "function", type(pal_client.refresh_token))
assert_eq("setup type", "function", type(pal_client.setup))

__builtin.log("")
__builtin.log("=== AIOT Client Structure Tests ===")

local aiot = require("haivivi/aiot")

local aiot_client = aiot.new_client("https://dummy-aiot.example.com", dummy_auth)

assert_not_nil("aiot client created", aiot_client)

-- Check resource collections exist (using actual fields from aiot.luau)
assert_not_nil("projects collection", aiot_client.projects)
assert_not_nil("agents collection", aiot_client.agents)

-- Check projects collection methods
assert_eq("projects.list type", "function", type(aiot_client.projects.list))
assert_eq("projects.get type", "function", type(aiot_client.projects.get))
assert_eq("projects.create type", "function", type(aiot_client.projects.create))
assert_eq("projects.key type", "function", type(aiot_client.projects.key))
assert_eq("projects.doc type", "function", type(aiot_client.projects.doc))

__builtin.log("")
__builtin.log("=== Results ===")
__builtin.log("Passed:", passed)
__builtin.log("Failed:", failed)

if failed > 0 then
    error("Some tests failed")
end

__builtin.log("=== HTTP Unit Tests OK ===")
