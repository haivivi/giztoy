--!strict
-- HTTP client module for Haivivi SDK.

local http = {}

export type HttpClient = {
    base_url: string,
    default_headers: {[string]: string},
    auth_fn: (() -> string)?,
    
    request: (self: HttpClient, method: string, path: string, body: any?, query: {[string]: string}?) -> (any, string?),
    get: (self: HttpClient, path: string, query: {[string]: string}?) -> (any, string?),
    post: (self: HttpClient, path: string, body: any?) -> (any, string?),
    put: (self: HttpClient, path: string, body: any?) -> (any, string?),
    patch: (self: HttpClient, path: string, body: any?) -> (any, string?),
    delete: (self: HttpClient, path: string) -> (any, string?),
}

export type HttpOptions = {
    headers: {[string]: string}?,
    auth: (() -> string)?,
}

-- URL encode a string
local function url_encode(str: string): string
    local result = string.gsub(str, "([^%w%-%.%_%~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end)
    return result
end

-- Build query string from table
local function build_query_string(query: {[string]: string}): string
    local parts = {}
    for k, v in pairs(query) do
        table.insert(parts, url_encode(k) .. "=" .. url_encode(v))
    end
    return table.concat(parts, "&")
end

-- Create a new HTTP client
function http.new(base_url: string, options: HttpOptions?): HttpClient
    local opts = options or {}
    
    local client: HttpClient = {
        base_url = base_url,
        default_headers = opts.headers or {},
        auth_fn = opts.auth,
    } :: HttpClient
    
    function client:request(method: string, path: string, body: any?, query: {[string]: string}?): (any, string?)
        -- Build URL
        local url = self.base_url .. path
        if query and next(query) then
            url = url .. "?" .. build_query_string(query)
        end
        
        -- Build headers
        local headers: {[string]: string} = {}
        for k, v in pairs(self.default_headers) do
            headers[k] = v
        end
        
        -- Add auth header if available
        if self.auth_fn then
            local token = self.auth_fn()
            if token and token ~= "" then
                headers["Authorization"] = "Bearer " .. token
            end
        end
        
        -- Set content type for requests with body
        if body then
            headers["Content-Type"] = "application/json"
        end
        
        -- Build request
        local req: {url: string, method: string, headers: {[string]: string}, body: string?} = {
            url = url,
            method = method,
            headers = headers,
        }
        
        if body then
            req.body = __builtin.json_encode(body)
        end
        
        -- Execute request
        local resp = __builtin.http(req)
        
        if resp.err then
            return nil, resp.err
        end
        
        -- Parse response body as JSON if possible
        local data = nil
        if resp.body and resp.body ~= "" then
            data = __builtin.json_decode(resp.body)
            if data == nil then
                -- Not JSON, return raw body
                data = resp.body
            end
        end
        
        -- Check for HTTP errors
        if resp.status >= 400 then
            local err_msg = "HTTP " .. tostring(resp.status)
            if type(data) == "table" and data.message then
                err_msg = err_msg .. ": " .. tostring(data.message)
            elseif type(data) == "table" and data.error then
                err_msg = err_msg .. ": " .. tostring(data.error)
            end
            return data, err_msg
        end
        
        return data, nil
    end
    
    function client:get(path: string, query: {[string]: string}?): (any, string?)
        return self:request("GET", path, nil, query)
    end
    
    function client:post(path: string, body: any?): (any, string?)
        return self:request("POST", path, body, nil)
    end
    
    function client:put(path: string, body: any?): (any, string?)
        return self:request("PUT", path, body, nil)
    end
    
    function client:patch(path: string, body: any?): (any, string?)
        return self:request("PATCH", path, body, nil)
    end
    
    function client:delete(path: string): (any, string?)
        return self:request("DELETE", path, nil, nil)
    end
    
    return client
end

return http
