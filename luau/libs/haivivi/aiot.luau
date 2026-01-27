--!strict
-- AIOT SDK module for Haivivi.

local http = require("haivivi/http")
local auth = require("haivivi/auth")
local resource = require("haivivi/resource")

local aiot = {}

-- Resource types
export type Project = {
    id: string,
    key: string,
    name: string?,
    desc: string?,
    owner: string,
    shortId: string,
    collaborators: {string}?,
}

export type Gear = {
    id: string,
    projectId: string,
    sn: string,
    eid: string,
    bindTo: string?,
    userScope: string?,
    firstSeenAt: string?,
    lastSeenAt: string?,
}

export type Agent = {
    id: string,
    projectId: string,
    gearId: string?,
    data: string?,
}

export type GearSignToken = {
    token: string,
    key: string,
    keyId: string,
    expiryUnixSec: number,
}

export type GearState = {
    state: any?,
    stats: any?,
}

-- Project document (for accessing sub-resources)
export type ProjectDoc = {
    id: string,
    http_client: http.HttpClient,
    
    get: (self: ProjectDoc) -> (Project?, string?),
    update: (self: ProjectDoc, dto: any) -> (Project?, string?),
    
    -- Sub-resources
    gears: GearsCollection,
    agents: AgentsCollection,
}

-- Gears collection under a project
export type GearsCollection = {
    http_client: http.HttpClient,
    project_id: string,
    
    list: (self: GearsCollection, options: any?) -> ({Gear}?, string?),
    get: (self: GearsCollection, id: string) -> (Gear?, string?),
    create: (self: GearsCollection, dto: any) -> (Gear?, string?),
    update: (self: GearsCollection, id: string, dto: any) -> (Gear?, string?),
    delete: (self: GearsCollection, id: string) -> (boolean, string?),
    
    -- Special methods
    get_by_sn: (self: GearsCollection, sn: string) -> (Gear?, string?),
    sn: (self: GearsCollection, sn: string) -> GearDoc,
    doc: (self: GearsCollection, id: string) -> GearDoc,
}

-- Gear document
export type GearDoc = {
    id: string,
    http_client: http.HttpClient,
    project_id: string,
    
    get: (self: GearDoc) -> (Gear?, string?),
    update: (self: GearDoc, dto: any) -> (Gear?, string?),
    state: (self: GearDoc) -> (GearState?, string?),
    sign_token: (self: GearDoc) -> (GearSignToken?, string?),
}

-- Agents collection under a project
export type AgentsCollection = {
    http_client: http.HttpClient,
    project_id: string,
    
    list: (self: AgentsCollection, options: any?) -> ({Agent}?, string?),
    get: (self: AgentsCollection, id: string) -> (Agent?, string?),
    create: (self: AgentsCollection, dto: any) -> (Agent?, string?),
    update: (self: AgentsCollection, id: string, dto: any) -> (Agent?, string?),
    delete: (self: AgentsCollection, id: string) -> (boolean, string?),
    
    -- Special methods
    register: (self: AgentsCollection, gear_id: string) -> (Agent?, string?),
}

-- Projects collection with key access
export type ProjectsCollection = {
    http_client: http.HttpClient,
    
    list: (self: ProjectsCollection, options: any?) -> ({Project}?, string?),
    get: (self: ProjectsCollection, id: string) -> (Project?, string?),
    create: (self: ProjectsCollection, dto: any) -> (Project?, string?),
    update: (self: ProjectsCollection, id: string, dto: any) -> (Project?, string?),
    upsert: (self: ProjectsCollection, dto: any) -> (Project?, string?),
    delete: (self: ProjectsCollection, id: string) -> (boolean, string?),
    
    -- Access by key
    key: (self: ProjectsCollection, key: string) -> ProjectDoc,
    doc: (self: ProjectsCollection, id: string) -> ProjectDoc,
}

-- AIOT client type
export type AiotClient = {
    base_url: string,
    auth_client: auth.AuthClient,
    
    -- Resources
    projects: ProjectsCollection,
    agents: resource.ResourceCollection<Agent, any, any, any>,
}

-- Create a GearDoc
local function create_gear_doc(hcli: http.HttpClient, project_id: string, id: string): GearDoc
    local doc: GearDoc = {
        id = id,
        http_client = hcli,
        project_id = project_id,
    } :: GearDoc
    
    local base_path = "/projects/" .. project_id .. "/gears/" .. id
    
    function doc:get(): (Gear?, string?)
        return hcli:get(base_path)
    end
    
    function doc:update(dto: any): (Gear?, string?)
        return hcli:patch(base_path, dto)
    end
    
    function doc:state(): (GearState?, string?)
        return hcli:get(base_path .. "/state")
    end
    
    function doc:sign_token(): (GearSignToken?, string?)
        return hcli:post(base_path .. "/@sign-token", nil)
    end
    
    return doc
end

-- Create a GearsCollection
local function create_gears_collection(hcli: http.HttpClient, project_id: string): GearsCollection
    local col: GearsCollection = {
        http_client = hcli,
        project_id = project_id,
    } :: GearsCollection
    
    local base_path = "/projects/" .. project_id .. "/gears"
    
    function col:list(options: any?): ({Gear}?, string?)
        local query: {[string]: string} = {}
        if options then
            for k, v in pairs(options) do
                if type(v) == "string" then
                    query[k] = v
                elseif type(v) == "number" then
                    query[k] = tostring(v)
                end
            end
        end
        return hcli:get(base_path, query)
    end
    
    function col:get(id: string): (Gear?, string?)
        return hcli:get(base_path .. "/" .. id)
    end
    
    function col:create(dto: any): (Gear?, string?)
        return hcli:post(base_path, dto)
    end
    
    function col:update(id: string, dto: any): (Gear?, string?)
        return hcli:patch(base_path .. "/" .. id, dto)
    end
    
    function col:delete(id: string): (boolean, string?)
        local _, err = hcli:delete(base_path .. "/" .. id)
        return err == nil, err
    end
    
    function col:get_by_sn(sn: string): (Gear?, string?)
        return hcli:post(base_path .. "/@getBySn", { sn = sn })
    end
    
    function col:sn(sn: string): GearDoc
        return create_gear_doc(hcli, project_id, "sn:" .. sn)
    end
    
    function col:doc(id: string): GearDoc
        return create_gear_doc(hcli, project_id, id)
    end
    
    return col
end

-- Create an AgentsCollection
local function create_agents_collection(hcli: http.HttpClient, project_id: string): AgentsCollection
    local col: AgentsCollection = {
        http_client = hcli,
        project_id = project_id,
    } :: AgentsCollection
    
    local base_path = "/projects/" .. project_id .. "/agents"
    
    function col:list(options: any?): ({Agent}?, string?)
        local query: {[string]: string} = {}
        if options then
            for k, v in pairs(options) do
                if type(v) == "string" then
                    query[k] = v
                end
            end
        end
        return hcli:get(base_path, query)
    end
    
    function col:get(id: string): (Agent?, string?)
        return hcli:get(base_path .. "/" .. id)
    end
    
    function col:create(dto: any): (Agent?, string?)
        return hcli:post(base_path, dto)
    end
    
    function col:update(id: string, dto: any): (Agent?, string?)
        return hcli:patch(base_path .. "/" .. id, dto)
    end
    
    function col:delete(id: string): (boolean, string?)
        local _, err = hcli:delete(base_path .. "/" .. id)
        return err == nil, err
    end
    
    function col:register(gear_id: string): (Agent?, string?)
        return hcli:post(base_path .. "/@register", { gearId = gear_id })
    end
    
    return col
end

-- Create a ProjectDoc
local function create_project_doc(hcli: http.HttpClient, id: string): ProjectDoc
    local doc: ProjectDoc = {
        id = id,
        http_client = hcli,
    } :: ProjectDoc
    
    local base_path = "/projects/" .. id
    
    function doc:get(): (Project?, string?)
        return hcli:get(base_path)
    end
    
    function doc:update(dto: any): (Project?, string?)
        return hcli:patch(base_path, dto)
    end
    
    -- Initialize sub-resources
    doc.gears = create_gears_collection(hcli, id)
    doc.agents = create_agents_collection(hcli, id)
    
    return doc
end

-- Create a ProjectsCollection
local function create_projects_collection(hcli: http.HttpClient): ProjectsCollection
    local col: ProjectsCollection = {
        http_client = hcli,
    } :: ProjectsCollection
    
    function col:list(options: any?): ({Project}?, string?)
        local query: {[string]: string} = {}
        if options then
            for k, v in pairs(options) do
                if type(v) == "string" then
                    query[k] = v
                end
            end
        end
        return hcli:get("/projects", query)
    end
    
    function col:get(id: string): (Project?, string?)
        return hcli:get("/projects/" .. id)
    end
    
    function col:create(dto: any): (Project?, string?)
        return hcli:post("/projects", dto)
    end
    
    function col:update(id: string, dto: any): (Project?, string?)
        return hcli:patch("/projects/" .. id, dto)
    end
    
    function col:upsert(dto: any): (Project?, string?)
        return hcli:put("/projects", dto)
    end
    
    function col:delete(id: string): (boolean, string?)
        local _, err = hcli:delete("/projects/" .. id)
        return err == nil, err
    end
    
    function col:key(key: string): ProjectDoc
        return create_project_doc(hcli, key)
    end
    
    function col:doc(id: string): ProjectDoc
        return create_project_doc(hcli, id)
    end
    
    return col
end

-- Create a new AIOT client
function aiot.new_client(base_url: string, auth_client: auth.AuthClient): AiotClient
    local hcli = http.new(base_url, {
        auth = function()
            return auth_client:get_token()
        end
    })
    
    local client: AiotClient = {
        base_url = base_url,
        auth_client = auth_client,
        
        projects = create_projects_collection(hcli),
        agents = resource.new_collection(hcli, "agents"),
    } :: AiotClient
    
    return client
end

return aiot
