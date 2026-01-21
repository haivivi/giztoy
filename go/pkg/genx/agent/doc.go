// Package agent provides a framework for building LLM-powered autonomous agents.
//
// # Overview
//
// The agent package implements a flexible agent architecture that supports:
//   - Multi-turn conversations with memory management
//   - Tool orchestration and execution
//   - Multiple agent types (ReAct, Match)
//
// # Agent Types
//
// ReActAgent implements the Reasoning and Acting (ReAct) pattern:
//   - Thinks step-by-step about user requests
//   - Selects and executes tools to accomplish tasks
//
// MatchAgent implements intent-based routing:
//   - Matches user input against predefined rules
//   - Routes to appropriate sub-agents or actions
//   - Useful for building multi-skill assistants
//
// # Event-Based API
//
// The Agent.Next() method returns AgentEvent for fine-grained control:
//
//	for {
//	    evt, err := agent.Next()
//	    if err != nil {
//	        return err
//	    }
//	    switch evt.Type {
//	    case EventChunk:
//	        // Normal output chunk
//	        fmt.Print(evt.Chunk.Part)
//	    case EventEOF:
//	        // Round ended, provide new input
//	        agent.Input(genx.Contents{genx.Text(userInput)})
//	    case EventClosed:
//	        // Agent completed (quit tool called) or closed
//	        return nil
//	    case EventToolStart:
//	        // Tool execution started
//	    case EventToolDone:
//	        // Tool completed successfully
//	    case EventToolError:
//	        // Tool execution failed
//	    case EventInterrupted:
//	        // Agent was interrupted
//	        return nil
//	    }
//	}
//
// # Quit Tools
//
// Tools can be marked as "quit tools" to signal agent completion:
//
//	tools:
//	  - $ref: tool:goodbye
//	    quit: true
//
// When a quit tool is executed, the agent finishes and returns EventClosed.
//
// # Tool System
//
// The package provides several tool types:
//   - BuiltinTool: Wraps Go functions as tools
//   - GeneratorTool: LLM-based text/JSON generation
//   - FinalizerTool: Structured output with validation
//   - HTTPTool: HTTP requests with jq-based response extraction
//   - CompositeTool: Sequential tool orchestration
//
// # Definition System
//
// Agent and tool configurations can be defined using:
//   - JSON for programmatic configuration
//   - YAML for human-readable configuration
//   - MessagePack for efficient binary serialization
//
// Definitions support references ($ref) for reusable components.
//
// # Runtime Interface
//
// The Runtime interface provides dependency injection for:
//   - LLM generation (via genx.Generator)
//   - Tool registry and creation
//   - Agent definition loading
//   - State management with memory capabilities
//
// # Example: Multi-Skill Assistant
//
// This example demonstrates a router agent that delegates to specialized sub-agents:
//
//	+-------------------------------------------------------+
//	|               Router Agent (Match)                    |
//	|       Rules: chat, fortune, music                     |
//	+------------+------------------+-----------------------+
//	             |                  |                       |
//	             v                  v                       v
//	    +----------------+  +----------------+  +----------------+
//	    |  Chat Agent    |  | Fortune Agent  |  |  Music Agent   |
//	    +----------------+  +-------+--------+  +-------+--------+
//	                                |                   |
//	                        +-------+-------+     +-----+-----+
//	                        | lunar_cal     |     | search    |
//	                        | calculate     |     | play      |
//	                        +---------------+     +-----------+
//
// Match Rules (using genx/match package):
//
// Rules are defined in YAML/JSON format with the following structure:
//
//	name: music
//	vars:
//	  title:
//	    label: 歌曲名
//	    type: string
//	  artist:
//	    label: 歌手
//	    type: string
//	patterns:
//	  - 播放歌曲
//	  - 我想听歌
//	  - ["我想听[title]", "title=[歌曲名]"]
//	  - ["我想听[artist]的歌", "artist=[歌手]"]
//	  - ["我想听[artist]的[title]", "artist=[歌手], title=[歌曲名]"]
//
// Pattern format:
//   - Simple string: "播放歌曲" (no variables)
//   - Array [input, output]: ["我想听[title]", "title=[歌曲名]"]
//
// The matcher outputs: "music: artist=周杰伦, title=稻香"
//
// Fortune Agent (ReAct):
//
//	type: react
//	name: fortune
//	prompt: |
//	  你是命理师。先收集用户的生辰八字信息，然后调用工具计算。
//	generator:
//	  model: gpt-4
//	tools:
//	  - $ref: tool:lunar_calendar
//	  - $ref: tool:calculate_fortune
//
// Music Agent (ReAct):
//
//	type: react
//	name: music
//	prompt: |
//	  你是音乐助手。使用 search 搜索歌曲，然后 play 播放。
//	generator:
//	  model: gpt-4
//	tools:
//	  - $ref: tool:search_song
//	  - $ref: tool:play_song
//
// Example Conversation:
//
//	User: 帮我算算命
//	Router: [matches "fortune" rule]
//	Fortune Agent: 请问您的出生日期和时辰？
//	User: 1990年5月15日早上8点
//	Fortune Agent: [calls lunar_calendar, calculate_fortune]
//	Fortune Agent: 根据您的八字分析...
//
//	User: 放首周杰伦的歌
//	Router: [matches "music: artist=周杰伦"]
//	Music Agent: [calls search_song]
//	Music Agent: 找到：1.青花瓷 2.稻香 3.晴天
//	User: 稻香
//	Music Agent: [calls play_song]
//	Music Agent: 正在播放：周杰伦 - 稻香
package agent
