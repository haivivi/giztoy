--!strict
-- Resource collection module for CRUD operations.

local http = require("haivivi/http")

local resource = {}

export type ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions> = {
    http_client: http.HttpClient,
    path: string,
    
    list: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, options: ListOptions?) -> ({T}?, string?),
    get: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, id: string) -> (T?, string?),
    create: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, dto: CreateDTO) -> (T?, string?),
    update: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, id: string, dto: UpdateDTO) -> (T?, string?),
    delete: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, id: string) -> (boolean, string?),
    
    -- Custom verbs
    post_verb: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, verb: string, body: any?) -> (any, string?),
    get_verb: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, id: string, verb: string) -> (any, string?),
    post_doc_verb: (self: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>, id: string, verb: string, body: any?) -> (any, string?),
}

-- Create a new resource collection
function resource.new_collection<T, CreateDTO, UpdateDTO, ListOptions>(
    http_client: http.HttpClient,
    path: string
): ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>
    local col: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions> = {
        http_client = http_client,
        path = path,
    } :: ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>
    
    function col:list(options: ListOptions?): ({T}?, string?)
        local query: {[string]: string} = {}
        if options then
            for k, v in pairs(options :: any) do
                if type(v) == "string" then
                    query[k] = v
                elseif type(v) == "number" then
                    query[k] = tostring(v)
                elseif type(v) == "boolean" then
                    query[k] = v and "true" or "false"
                end
            end
        end
        return self.http_client:get("/" .. self.path, query)
    end
    
    function col:get(id: string): (T?, string?)
        return self.http_client:get("/" .. self.path .. "/" .. id)
    end
    
    function col:create(dto: CreateDTO): (T?, string?)
        return self.http_client:post("/" .. self.path, dto)
    end
    
    function col:update(id: string, dto: UpdateDTO): (T?, string?)
        return self.http_client:patch("/" .. self.path .. "/" .. id, dto)
    end
    
    function col:delete(id: string): (boolean, string?)
        local _, err = self.http_client:delete("/" .. self.path .. "/" .. id)
        return err == nil, err
    end
    
    function col:post_verb(verb: string, body: any?): (any, string?)
        return self.http_client:post("/" .. self.path .. "/" .. verb, body)
    end
    
    function col:get_verb(id: string, verb: string): (any, string?)
        return self.http_client:get("/" .. self.path .. "/" .. id .. "/" .. verb)
    end
    
    function col:post_doc_verb(id: string, verb: string, body: any?): (any, string?)
        return self.http_client:post("/" .. self.path .. "/" .. id .. "/" .. verb, body)
    end
    
    return col
end

-- Create a sub-resource collection (nested under a parent)
function resource.new_sub_collection<T, CreateDTO, UpdateDTO, ListOptions>(
    http_client: http.HttpClient,
    parent_path: string,
    parent_id: string,
    path: string
): ResourceCollection<T, CreateDTO, UpdateDTO, ListOptions>
    local full_path = parent_path .. "/" .. parent_id .. "/" .. path
    return resource.new_collection(http_client, full_path)
end

return resource
