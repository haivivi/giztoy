package agent_test

import (
	"encoding/json"
	"fmt"

	"github.com/haivivi/giztoy/pkg/genx/agent"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
)

// Example_reActAgentDef demonstrates how to define a ReAct agent using JSON.
func Example_reActAgentDef() {
	jsonDef := `{
		"type": "react",
		"name": "assistant",
		"generator": {
			"model": "gpt-4"
		},
		"tools": [
			{"$ref": "tool:search"},
			{"$ref": "tool:calculator"}
		]
	}`

	var def agentcfg.ReActAgent
	if err := json.Unmarshal([]byte(jsonDef), &def); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Agent name:", def.Name)
	fmt.Println("Model:", def.Generator.Generator.Model)
	fmt.Println("Tools count:", len(def.Tools))
	// Output:
	// Agent name: assistant
	// Model: gpt-4
	// Tools count: 2
}

// Example_matchAgentDef demonstrates how to define a Match agent using JSON.
func Example_matchAgentDef() {
	jsonDef := `{
		"type": "match",
		"name": "router",
		"generator": {
			"model": "gpt-4"
		},
		"rules": [
			{
				"$ref": "rule:play_music"
			}
		]
	}`

	var def agentcfg.MatchAgent
	if err := json.Unmarshal([]byte(jsonDef), &def); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Agent name:", def.Name)
	fmt.Println("Model:", def.Generator.Generator.Model)
	fmt.Println("Rules count:", len(def.Rules))
	// Output:
	// Agent name: router
	// Model: gpt-4
	// Rules count: 1
}

// Example_generatorToolDef demonstrates how to define a generator tool.
func Example_generatorToolDef() {
	jsonDef := `{
		"type": "generator",
		"name": "summarize",
		"description": "Summarize the given text",
		"model": "gpt-4",
		"mode": "generate",
		"prompt": "You are a helpful assistant that summarizes text concisely."
	}`

	def, err := agentcfg.UnmarshalTool([]byte(jsonDef))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	genTool := agentcfg.AsGeneratorTool(def)
	fmt.Println("Tool name:", genTool.Name)
	fmt.Println("Tool type:", genTool.Type)
	// Output:
	// Tool name: summarize
	// Tool type: generator
}

// Example_compositeToolDef demonstrates how to define a composite tool
// that executes multiple steps in sequence.
func Example_compositeToolDef() {
	jsonDef := `{
		"type": "composite",
		"name": "search_and_summarize",
		"description": "Search for information and summarize results",
		"steps": [
			{
				"id": "search",
				"tool": {"$ref": "tool:web_search"},
				"input": "${{ input }}"
			},
			{
				"id": "summarize",
				"tool": {"$ref": "tool:summarize"},
				"input": "${{ steps.search.output }}"
			}
		]
	}`

	def, err := agentcfg.UnmarshalTool([]byte(jsonDef))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	compTool := agentcfg.AsCompositeTool(def)
	fmt.Println("Tool name:", compTool.Name)
	fmt.Println("Steps count:", len(compTool.Steps))
	fmt.Println("First step ID:", compTool.Steps[0].ID)
	// Output:
	// Tool name: search_and_summarize
	// Steps count: 2
	// First step ID: search
}

// Example_httpToolDef demonstrates how to define an HTTP tool.
func Example_httpToolDef() {
	jsonDef := `{
		"type": "http",
		"name": "get_weather",
		"description": "Get weather for a location",
		"method": "GET",
		"endpoint": "https://api.weather.com/v1/forecast",
		"resp_body_jq": ".forecast.daily[0]"
	}`

	def, err := agentcfg.UnmarshalTool([]byte(jsonDef))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	httpTool := agentcfg.AsHTTPTool(def)
	fmt.Println("Tool name:", httpTool.Name)
	fmt.Println("Method:", httpTool.Method)
	fmt.Println("Has JQ filter:", httpTool.RespBodyJQ != nil)
	// Output:
	// Tool name: get_weather
	// Method: GET
	// Has JQ filter: true
}

// Example_contextLayers demonstrates different types of context layers.
func Example_contextLayers() {
	jsonDef := `{
		"type": "react",
		"name": "assistant",
		"generator": {"model": "gpt-4"},
		"context_layers": [
			"You are a helpful assistant.",
			{"$ref": "context:character_alice"},
			{"$env": "SYSTEM_PROMPT"},
			{"$mem": {"summary": true, "recent": 10}}
		]
	}`

	var def agentcfg.ReActAgent
	if err := json.Unmarshal([]byte(jsonDef), &def); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Context layers count:", len(def.ContextLayers))
	// Output:
	// Context layers count: 4
}

// Example_quitTool demonstrates how to define a quit tool that signals agent completion.
func Example_quitTool() {
	jsonDef := `{
		"type": "react",
		"name": "assistant",
		"generator": {"model": "gpt-4"},
		"tools": [
			{"$ref": "tool:search"},
			{"$ref": "tool:goodbye", "quit": true}
		]
	}`

	var def agentcfg.ReActAgent
	if err := json.Unmarshal([]byte(jsonDef), &def); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Tools count:", len(def.Tools))
	fmt.Println("Second tool is quit:", def.Tools[1].Quit)
	// Output:
	// Tools count: 2
	// Second tool is quit: true
}

// Example_eventTypes demonstrates the AgentEvent types.
func Example_eventTypes() {
	// EventType constants for handling Agent.Next() results
	fmt.Println("EventChunk:", agent.EventChunk)
	fmt.Println("EventEOF:", agent.EventEOF)
	fmt.Println("EventClosed:", agent.EventClosed)
	fmt.Println("EventToolStart:", agent.EventToolStart)
	fmt.Println("EventToolDone:", agent.EventToolDone)
	fmt.Println("EventToolError:", agent.EventToolError)
	fmt.Println("EventInterrupted:", agent.EventInterrupted)
	// Output:
	// EventChunk: chunk
	// EventEOF: eof
	// EventClosed: closed
	// EventToolStart: tool_start
	// EventToolDone: tool_done
	// EventToolError: tool_error
	// EventInterrupted: interrupted
}
