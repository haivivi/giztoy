--!strict
--[[
    Type declarations for the runtime context (rt) global.
    
    The 'rt' table is injected by the Go runtime and provides:
    - Agent context: recv/emit for streaming I/O
    - Tool context: input/output for one-shot I/O
    
    Usage:
        -- Agent script
        local chunk, err = rt:recv()
        rt:emit({ part = { type = "text", value = "hello" } })
        
        -- Tool script
        local input = rt:input()
        rt:output({ result = "done" }, nil)
]]

-- Part represents a content part in a message.
export type Part = {
    type: string,      -- "text" | "audio" | "function_call" | etc.
    value: any?,       -- Content value (type depends on part type)
    id: string?,       -- Optional part ID
}

-- Usage represents token usage statistics.
export type Usage = {
    inputTokens: number?,
    outputTokens: number?,
}

-- MessageChunk represents a streaming message chunk.
export type MessageChunk = {
    part: Part?,       -- The content part
    index: number?,    -- Part index (for multi-part messages)
    event: string?,    -- Event type: "start" | "delta" | "end"
    usage: Usage?,     -- Token usage statistics
}

-- RuntimeContext provides the rt table interface.
export type RuntimeContext = {
    -- Agent methods (streaming I/O)
    recv: (self: RuntimeContext) -> (MessageChunk?, string?),
    emit: (self: RuntimeContext, chunk: MessageChunk) -> string?,
    
    -- Tool methods (one-shot I/O)
    input: (self: RuntimeContext) -> any,
    output: (self: RuntimeContext, result: any, err: string?) -> (),
}

-- Global rt table, injected by Go runtime.
-- Available when running in Agent or Tool context.
declare rt: RuntimeContext
