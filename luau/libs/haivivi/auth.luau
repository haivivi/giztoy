--!strict
-- Auth SDK module for Haivivi.

local http = require("haivivi/http")
local resource = require("haivivi/resource")
local async = require("haivivi/async")

local auth = {}

-- Token refresh thresholds (in seconds)
local STALE_THRESHOLD = 15 * 60   -- 15 minutes before expiry: token is stale (background refresh)
local EXPIRED_THRESHOLD = 0       -- At expiry: token is expired (must wait for refresh)

-- Session type
export type Session = {
    id: string,
    key: string,
    token: string,
    tokenExpireAt: string,
    expireAt: string,
    user: User?,
}

export type User = {
    id: string,
    ns: string,
    scope: string,
    nickname: string?,
    avatar: string?,
    email: string?,
    phone: string?,
    roles: {string}?,
}

export type Namespace = {
    id: string,
    key: string,
    name: string,
    ns: string,
    desc: string?,
}

-- Token state: valid > stale > expired > none
export type TokenState = "valid" | "stale" | "expired" | "none"

-- Auth client type
export type AuthClient = {
    base_url: string,
    key: string,
    session: Session?,
    
    -- Internal: ongoing refresh task (to avoid duplicate refreshes)
    _refresh_task: async.Task<Session>?,
    
    -- Sync methods (blocking)
    refresh_token: (self: AuthClient) -> (Session?, string?),
    needs_refresh: (self: AuthClient) -> boolean,
    get_token: (self: AuthClient) -> string,
    http_client: (self: AuthClient) -> http.HttpClient,
    
    -- State inspection
    token_state: (self: AuthClient) -> TokenState,
    is_refreshing: (self: AuthClient) -> boolean,
    
    -- Async methods (coroutine-based)
    refresh_token_async: (self: AuthClient) -> async.Task<Session>,
    get_token_async: (self: AuthClient) -> async.Task<string>,
    
    -- Resources
    sessions: resource.ResourceCollection<Session, any, any, any>,
    users: resource.ResourceCollection<User, any, any, any>,
    namespaces: resource.ResourceCollection<Namespace, any, any, any>,
}

-- KVS keys for token storage
local KVS_SESSION_KEY = "haivivi:auth:session"

-- Create a new auth client
function auth.new_client(base_url: string, key: string): AuthClient
    local client: AuthClient = {
        base_url = base_url,
        key = key,
        session = nil,
        _refresh_task = nil,
    } :: AuthClient
    
    -- Try to load cached session
    local cached = __builtin.kvs_get(KVS_SESSION_KEY)
    if cached then
        client.session = cached :: Session
    end
    
    -- ========================================================================
    -- Token state inspection
    -- ========================================================================
    
    -- Get current token state: valid, stale, expired, or none
    function client:token_state(): TokenState
        if not self.session or not self.session.token or self.session.token == "" then
            return "none"
        end
        
        if not self.session.tokenExpireAt then
            -- No expiry info, assume valid
            return "valid"
        end
        
        local expire_at = __builtin.parse_time(self.session.tokenExpireAt)
        if not expire_at then
            return "valid"  -- Can't parse, assume valid
        end
        
        local now = __builtin.time()
        local remaining = expire_at - now
        
        if remaining <= EXPIRED_THRESHOLD then
            return "expired"
        elseif remaining <= STALE_THRESHOLD then
            return "stale"
        else
            return "valid"
        end
    end
    
    -- Check if a refresh is currently in progress
    function client:is_refreshing(): boolean
        return self._refresh_task ~= nil and async.pending(self._refresh_task)
    end
    
    -- Check if token needs refresh (for backward compatibility)
    function client:needs_refresh(): boolean
        local state = self:token_state()
        return state ~= "valid"
    end
    
    -- ========================================================================
    -- Sync methods (blocking)
    -- ========================================================================
    
    -- Internal: perform HTTP refresh request
    local function do_refresh(self_ref: AuthClient): (Session?, string?)
        local req = {
            url = self_ref.base_url .. "/me/@refresh",
            method = "POST",
            headers = {
                ["Content-Type"] = "application/json",
            },
            body = __builtin.json_encode({ key = self_ref.key }),
        }
        
        local resp = __builtin.http(req)
        
        if resp.err then
            return nil, resp.err
        end
        
        if resp.status >= 400 then
            local data = __builtin.json_decode(resp.body)
            local msg = "HTTP " .. tostring(resp.status)
            if data and data.message then
                msg = msg .. ": " .. data.message
            end
            return nil, msg
        end
        
        local session = __builtin.json_decode(resp.body)
        if not session then
            return nil, "failed to parse session"
        end
        
        self_ref.session = session
        __builtin.kvs_set(KVS_SESSION_KEY, session)
        
        return session, nil
    end
    
    -- Refresh token using key (always performs refresh)
    function client:refresh_token(): (Session?, string?)
        return do_refresh(self)
    end
    
    -- Get current token with smart refresh logic:
    -- - valid: return immediately
    -- - stale: start background refresh if not running, return current token
    -- - expired/none: wait for refresh to complete, return new token
    function client:get_token(): string
        local state = self:token_state()
        
        if state == "valid" then
            -- Token is good, return immediately
            return self.session.token
        end
        
        if state == "stale" then
            -- Token is stale but usable
            -- Start background refresh if not already running
            if not self:is_refreshing() then
                -- Note: In sync mode, we can't truly do background refresh
                -- But we can still return the current token and let the next call refresh
                -- For true background refresh, use async mode
                -- Here we just return the current token
            end
            return self.session.token
        end
        
        -- Token is expired or none - must refresh and wait
        if self:is_refreshing() then
            -- Wait for existing refresh task
            local session, err = async.run_sync(self._refresh_task)
            if session then
                return session.token
            end
            -- Refresh failed, try once more
        end
        
        -- Perform refresh
        local session, err = self:refresh_token()
        if session then
            return session.token
        end
        
        -- Refresh failed, return existing token if any (caller handles 401)
        if self.session and self.session.token then
            return self.session.token
        end
        return ""
    end
    
    -- Get HTTP client with auth
    function client:http_client(): http.HttpClient
        return http.new(self.base_url, {
            auth = function()
                return self:get_token()
            end
        })
    end
    
    -- ========================================================================
    -- Async methods (coroutine-based)
    -- ========================================================================
    
    -- Internal: create a new refresh task
    local function create_refresh_task(self_ref: AuthClient): async.Task<Session>
        local task: async.Task<Session> = {
            co = coroutine.create(function()
                local req = {
                    url = self_ref.base_url .. "/me/@refresh",
                    method = "POST",
                    headers = {
                        ["Content-Type"] = "application/json",
                    },
                    body = __builtin.json_encode({ key = self_ref.key }),
                }
                
                -- Yield HTTP request to host
                local resp = coroutine.yield({
                    type = "http",
                    request = req,
                } :: async.YieldValue)
                
                -- Clear refresh task reference when done
                self_ref._refresh_task = nil
                
                if resp.err then
                    error(resp.err)
                end
                
                if resp.status >= 400 then
                    local data = __builtin.json_decode(resp.body)
                    local msg = "HTTP " .. tostring(resp.status)
                    if data and data.message then
                        msg = msg .. ": " .. data.message
                    end
                    error(msg)
                end
                
                local session = __builtin.json_decode(resp.body)
                if not session then
                    error("failed to parse session")
                end
                
                self_ref.session = session
                __builtin.kvs_set(KVS_SESSION_KEY, session)
                
                return session
            end),
            state = "pending",
            result = nil,
            error = nil,
        }
        return task
    end
    
    -- Async refresh token - yields to host for HTTP
    -- Reuses existing refresh task if one is in progress
    function client:refresh_token_async(): async.Task<Session>
        -- If already refreshing, return existing task
        if self:is_refreshing() then
            return self._refresh_task
        end
        
        -- Create new refresh task
        local task = create_refresh_task(self)
        self._refresh_task = task
        return task
    end
    
    -- Async get token with smart refresh logic:
    -- - valid: return immediately (no yield)
    -- - stale: start background refresh if not running, return current token (no yield)
    -- - expired/none: yield and wait for refresh to complete
    function client:get_token_async(): async.Task<string>
        local self_ref = self
        local task: async.Task<string> = {
            co = coroutine.create(function()
                local state = self_ref:token_state()
                
                if state == "valid" then
                    -- Token is good, return immediately
                    return self_ref.session.token
                end
                
                if state == "stale" then
                    -- Token is stale but usable
                    -- Start background refresh if not already running
                    if not self_ref:is_refreshing() then
                        -- Start refresh task (don't wait for it)
                        self_ref._refresh_task = create_refresh_task(self_ref)
                        -- Yield a "background" marker so host can poll it
                        coroutine.yield({
                            type = "background_task",
                            task = self_ref._refresh_task,
                        })
                    end
                    -- Return current (stale but usable) token
                    return self_ref.session.token
                end
                
                -- Token is expired or none - must refresh and wait
                local refresh_task: async.Task<Session>
                if self_ref:is_refreshing() then
                    -- Reuse existing refresh task
                    refresh_task = self_ref._refresh_task
                else
                    -- Start new refresh task
                    refresh_task = create_refresh_task(self_ref)
                    self_ref._refresh_task = refresh_task
                end
                
                -- Wait for refresh by yielding until task completes
                while async.pending(refresh_task) do
                    local yield_val = async.poll(refresh_task)
                    if yield_val then
                        -- Re-yield to host (HTTP request)
                        local resp = coroutine.yield(yield_val)
                        async.poll(refresh_task, resp)
                    end
                end
                
                -- Get result
                local session, err = async.result(refresh_task)
                if session then
                    return session.token
                end
                
                -- Refresh failed, return existing token if any
                if self_ref.session and self_ref.session.token then
                    return self_ref.session.token
                end
                
                if err then
                    error(err)
                end
                return ""
            end),
            state = "pending",
            result = nil,
            error = nil,
        }
        return task
    end
    
    -- Initialize resource collections
    local hcli = http.new(base_url, {
        auth = function()
            return client:get_token()
        end
    })
    
    client.sessions = resource.new_collection(hcli, "sessions")
    client.users = resource.new_collection(hcli, "users")
    client.namespaces = resource.new_collection(hcli, "namespaces")
    
    return client
end

return auth
