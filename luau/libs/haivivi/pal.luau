--!strict
-- PAL (Platform Abstraction Layer) SDK module for Haivivi.

local http = require("haivivi/http")
local auth = require("haivivi/auth")
local resource = require("haivivi/resource")

local pal = {}

-- Device Auth type
export type DeviceAuth = {
    token: string,
    tokenExpireAt: string,
    key: string,
}

-- Resource types
export type Character = {
    id: string,
    name: string,
    type: string,
    desc: string?,
    avatar: string?,
    prompt: string?,
    sampleVoice: string?,
    mediaAssets: any?,
}

export type Voice = {
    id: string,
    name: string,
    provider: string?,
    voiceId: string?,
    language: string?,
    gender: string?,
    desc: string?,
}

export type ChatTopic = {
    id: string,
    name: string,
    desc: string?,
    prompt: string?,
    mediaAssets: any?,
}

export type VirtualDevice = {
    id: string,
    uid: string,
    name: string?,
    characterId: string?,
    voiceId: string?,
    chatTopicId: string?,
    gearId: string?,
    deviceSn: string?,
}

export type Album = {
    id: string,
    name: string,
    desc: string?,
    cover: string?,
}

export type Firmware = {
    id: string,
    version: string,
    build: number?,
    url: string?,
    releaseNotes: string?,
}

export type Trigger = {
    id: string,
    name: string,
    type: string?,
    prompts: any?,
}

export type TTSModel = {
    id: string,
    name: string,
    provider: string?,
}

export type TunedLLM = {
    id: string,
    name: string,
    provider: string?,
    model: string?,
}

export type Membership = {
    id: string,
    uid: string,
    planId: string?,
    status: string?,
}

export type Order = {
    id: string,
    uid: string,
    status: string?,
    totalAmount: number?,
}

export type Payment = {
    id: string,
    orderId: string?,
    status: string?,
    amount: number?,
}

export type Plan = {
    id: string,
    name: string,
    price: number?,
    interval: string?,
}

export type Subscription = {
    id: string,
    uid: string,
    planId: string?,
    status: string?,
}

export type Tag = {
    id: string,
    name: string,
    type: string?,
}

export type AccessPolicy = {
    id: string,
    scopeLevel: string?,
    scope: string?,
    resourceType: string?,
    resourceId: string?,
}

export type Achievement = {
    id: string,
    name: string,
    typeId: string?,
    desc: string?,
}

export type AchievementType = {
    id: string,
    name: string,
}

export type AchievementProgress = {
    id: string,
    achievementId: string?,
    virtualDeviceId: string?,
    progress: number?,
}

export type DeviceLog = {
    id: string,
    type: string?,
    source: string?,
    content: string?,
}

export type DeviceGiftCard = {
    id: string,
    code: string?,
    status: string?,
}

export type Campaign = {
    id: string,
    name: string,
    status: string?,
}

export type PresetPrompt = {
    id: string,
    name: string,
    prompt: string?,
}

export type Report = {
    id: string,
    type: string?,
    status: string?,
}

export type Series = {
    id: string,
    name: string,
    desc: string?,
}

-- PAL client type
export type PalClient = {
    base_url: string,
    auth_client: auth.AuthClient,
    
    -- Device auth methods
    refresh_token: (self: PalClient, key: string) -> (DeviceAuth?, string?),
    setup: (self: PalClient, uat: string, eid: string, vid: string) -> (DeviceAuth?, string?),
    
    -- Resource collections
    characters: resource.ResourceCollection<Character, any, any, any>,
    voices: resource.ResourceCollection<Voice, any, any, any>,
    chat_topics: resource.ResourceCollection<ChatTopic, any, any, any>,
    virtual_devices: resource.ResourceCollection<VirtualDevice, any, any, any>,
    albums: resource.ResourceCollection<Album, any, any, any>,
    firmwares: resource.ResourceCollection<Firmware, any, any, any>,
    triggers: resource.ResourceCollection<Trigger, any, any, any>,
    tts_models: resource.ResourceCollection<TTSModel, any, any, any>,
    tuned_llms: resource.ResourceCollection<TunedLLM, any, any, any>,
    memberships: resource.ResourceCollection<Membership, any, any, any>,
    orders: resource.ResourceCollection<Order, any, any, any>,
    payments: resource.ResourceCollection<Payment, any, any, any>,
    plans: resource.ResourceCollection<Plan, any, any, any>,
    subscriptions: resource.ResourceCollection<Subscription, any, any, any>,
    tags: resource.ResourceCollection<Tag, any, any, any>,
    access_policies: resource.ResourceCollection<AccessPolicy, any, any, any>,
    achievements: resource.ResourceCollection<Achievement, any, any, any>,
    achievement_types: resource.ResourceCollection<AchievementType, any, any, any>,
    achievement_progresses: resource.ResourceCollection<AchievementProgress, any, any, any>,
    device_logs: resource.ResourceCollection<DeviceLog, any, any, any>,
    device_gift_cards: resource.ResourceCollection<DeviceGiftCard, any, any, any>,
    campaigns: resource.ResourceCollection<Campaign, any, any, any>,
    preset_prompts: resource.ResourceCollection<PresetPrompt, any, any, any>,
    reports: resource.ResourceCollection<Report, any, any, any>,
    series: resource.ResourceCollection<Series, any, any, any>,
}

-- Create a new PAL client
function pal.new_client(base_url: string, auth_client: auth.AuthClient): PalClient
    local hcli = http.new(base_url, {
        auth = function()
            return auth_client:get_token()
        end
    })
    
    local client: PalClient = {
        base_url = base_url,
        auth_client = auth_client,
        
        -- Initialize all resource collections
        characters = resource.new_collection(hcli, "characters"),
        voices = resource.new_collection(hcli, "voices"),
        chat_topics = resource.new_collection(hcli, "chat-topics"),
        virtual_devices = resource.new_collection(hcli, "virtual-devices"),
        albums = resource.new_collection(hcli, "albums"),
        firmwares = resource.new_collection(hcli, "firmwares"),
        triggers = resource.new_collection(hcli, "triggers"),
        tts_models = resource.new_collection(hcli, "tts-models"),
        tuned_llms = resource.new_collection(hcli, "tuned-llms"),
        memberships = resource.new_collection(hcli, "memberships"),
        orders = resource.new_collection(hcli, "orders"),
        payments = resource.new_collection(hcli, "payments"),
        plans = resource.new_collection(hcli, "plans"),
        subscriptions = resource.new_collection(hcli, "subscriptions"),
        tags = resource.new_collection(hcli, "tags"),
        access_policies = resource.new_collection(hcli, "access-policies"),
        achievements = resource.new_collection(hcli, "achievements"),
        achievement_types = resource.new_collection(hcli, "achievement-types"),
        achievement_progresses = resource.new_collection(hcli, "achievement-progresses"),
        device_logs = resource.new_collection(hcli, "device-logs"),
        device_gift_cards = resource.new_collection(hcli, "device-gift-cards"),
        campaigns = resource.new_collection(hcli, "campaigns"),
        preset_prompts = resource.new_collection(hcli, "preset-prompts"),
        reports = resource.new_collection(hcli, "reports"),
        series = resource.new_collection(hcli, "series"),
    } :: PalClient
    
    -- Refresh device token
    function client:refresh_token(key: string): (DeviceAuth?, string?)
        local req = {
            url = self.base_url .. "/device-auth/refresh-token",
            method = "POST",
            headers = {
                ["Content-Type"] = "application/json",
            },
            body = __builtin.json_encode({ key = key }),
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
        
        return __builtin.json_decode(resp.body), nil
    end
    
    -- Setup device
    function client:setup(uat: string, eid: string, vid: string): (DeviceAuth?, string?)
        local req = {
            url = self.base_url .. "/device-auth/setup",
            method = "POST",
            headers = {
                ["Content-Type"] = "application/json",
                ["Authorization"] = "Bearer " .. uat,
            },
            body = __builtin.json_encode({ eid = eid, vid = vid }),
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
        
        return __builtin.json_decode(resp.body), nil
    end
    
    return client
end

return pal
