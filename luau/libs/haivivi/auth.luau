--!strict
-- Auth SDK module for Haivivi.

local http = require("haivivi/http")
local resource = require("haivivi/resource")

local auth = {}

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

-- Auth client type
export type AuthClient = {
    base_url: string,
    key: string,
    session: Session?,
    
    -- Methods
    refresh_token: (self: AuthClient) -> (Session?, string?),
    get_token: (self: AuthClient) -> string,
    http_client: (self: AuthClient) -> http.HttpClient,
    
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
    } :: AuthClient
    
    -- Try to load cached session
    local cached = __builtin.kvs_get(KVS_SESSION_KEY)
    if cached then
        client.session = cached :: Session
    end
    
    -- Refresh token using key
    function client:refresh_token(): (Session?, string?)
        local req = {
            url = self.base_url .. "/me/@refresh",
            method = "POST",
            headers = {
                ["Content-Type"] = "application/json",
            },
            body = __builtin.json_encode({ key = self.key }),
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
        
        self.session = session
        __builtin.kvs_set(KVS_SESSION_KEY, session)
        
        return session, nil
    end
    
    -- Get current token (refresh if needed)
    function client:get_token(): string
        if self.session and self.session.token then
            -- TODO: Check expiry and refresh if needed
            return self.session.token
        end
        
        -- Try to refresh
        local session, err = self:refresh_token()
        if session then
            return session.token
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
