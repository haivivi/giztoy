# GenX - Known Issues

## 🔴 Major Issues

### GX-001: Rust lacks agent framework

**Description:**  
Rust implementation missing the entire agent framework:
- No ReActAgent
- No MatchAgent
- No tool orchestration
- No configuration parser

**Impact:** Cannot build autonomous agents in Rust.

**Effort:** High - requires significant implementation work.

---

### GX-002: Rust lacks advanced tool types

**Description:**  
Rust only has `FuncTool`. Missing:
- `GeneratorTool`
- `HTTPTool`
- `CompositeTool`
- `TextProcessorTool`

**Impact:** Limited tool capabilities in Rust.

---

## 🟡 Minor Issues

### GX-003: Go agent uses panics for some errors

**File:** `go/pkg/genx/agent/agent.go`

**Description:**  
Some internal errors use panic instead of returning errors.

**Impact:** Can crash applications on unexpected states.

**Suggestion:** Convert panics in public entry points to errors; keep panics only for truly unreachable states.

---

### GX-004: Configuration parsing is complex

**Description:**  
The agentcfg package has complex unmarshal logic with many edge cases.

**Files:**
- `go/pkg/genx/agentcfg/unmarshal.go`
- `go/pkg/genx/agentcfg/*_unmarshal_test.go`

**Note:** Extensive tests exist, so this is well-covered.

---

### GX-006: Streaming tool-call collection parity uncertain

**Description:**  
Rust includes `collect_tool_calls_streamed`, but feature parity with Go streaming tool calls needs verification and tests.

---

---

### GX-019: MessageChunk.Clone drops tool calls

**File:** `go/pkg/genx/message.go`

**Description:**  
`MessageChunk.Clone()` copies `Role/Name/Part` but never copies `ToolCall`.

**Impact:** Tool-call chunks can be silently lost when cloned.

**Suggestion:** Copy `c.ToolCall` instead of checking `chk.ToolCall`.

---

### GX-020: StreamBuilder drops unknown tool calls

**File:** `go/pkg/genx/stream_builder.go`

**Description:**  
If a tool call references a tool not found in `ModelContext`, the chunk is skipped:

```go
if !ok { slog.Warn(...); continue }
```

**Impact:** Tool-call chunks disappear without being forwarded to consumers.

**Suggestion:** Emit the chunk anyway or return an error so callers can handle missing tools.

---

### GX-021: OpenAI Invoke drops usage metrics

**File:** `go/pkg/genx/openai.go`

**Description:**  
`invokeJSONOutput` and `invokeToolCalls` return `Usage{}` instead of `resp.Usage`.

**Impact:** Usage accounting is always zero for invoke paths.

**Suggestion:** Return `oaiConvUsage(&resp.Usage)` on success.

---

### GX-022: GenerateStream goroutine can leak on early close

**File:** `go/pkg/genx/openai.go`

**Description:**  
`GenerateStream` spawns a goroutine reading the OpenAI stream. If the caller closes
the stream early without cancelling the context, the goroutine may continue until
the server ends the stream.

**Impact:** Potential goroutine/resource leak in long-running sessions.

**Suggestion:** Tie stream close to context cancellation or add a stop channel.

---

### GX-023: hexString ignores rand.Read error

**File:** `go/pkg/genx/json.go`

**Description:**  
`rand.Read` errors are ignored when generating IDs.

**Impact:** On RNG failure, ID may be all-zero without error signal.

**Suggestion:** Check `rand.Read` error and fall back or return error.

---

## 🔵 Enhancements

### GX-007: Add more provider adapters

**Description:**  
Currently supports OpenAI and Gemini. Could add:
- Anthropic (Claude)
- Mistral
- Local models (Ollama)

---

### GX-008: Add retry logic to generators

**Description:**  
No built-in retry for transient failures.

**Suggestion:** Add configurable retry with backoff.

---

### GX-009: Add request/response logging

**Description:**  
No debug logging for API calls.

**Suggestion:** Add optional verbose mode.

---

### GX-010: Document match pattern syntax

**Description:**  
Match patterns have complex syntax; documentation exists but should stay in sync.

**Files:**  
- `docs/genx/match/`  
- `go/pkg/genx/match/`

---

### GX-011: Add validation for agent configs

**Description:**  
YAML configs could have invalid references (`$ref`). No validation until runtime.

**Suggestion:** Add config validation command/function.

---

### GX-012: Add configuration schema generation

**Description:**  
No JSON Schema is provided for agent/tool configuration.

**Impact:** No IDE auto-complete or static validation.

**Suggestion:** Generate JSON Schema from `agentcfg` types and publish under `docs/`.

---

### GX-013: Add stream test coverage for tool calls (Rust)

**Description:**  
Tool-call streaming helpers exist, but end-to-end tests are limited or missing.

**Impact:** Potential regressions in streamed tool-call parsing.

**Suggestion:** Add tests that simulate incremental chunks and verify parsed tool calls.

---

### GX-024: StreamBuilder::new ignores tools (Rust)

**File:** `rust/genx/src/stream.rs`

**Description:**  
`StreamBuilder::new` ignores tools from `ModelContext`, leaving `func_tools` empty.

**Impact:** Tool-call metadata cannot be linked unless callers use `with_tools`.

**Suggestion:** Provide a way to downcast tools or pass tool list explicitly in generator code.

---

## ⚪ Notes

### GX-014: Well-structured Go implementation

**Description:**  
The Go genx package is well-organized:
- Clear separation of concerns
- Extensive test coverage
- Comprehensive agent framework
- YAML/JSON configuration support

---

### GX-015: Event-based agent API

**Description:**  
The agent event system is well-designed:
```go
for {
    evt, err := ag.Next()
    switch evt.Type {
    case EventChunk: ...
    case EventToolStart: ...
    case EventClosed: ...
    }
}
```

Provides fine-grained control over agent execution.

---

### GX-016: Quit tool pattern

**Description:**  
Tools can be marked as "quit tools" to signal agent completion:
```yaml
tools:
  - $ref: tool:goodbye
    quit: true
```

Useful for conversational agents with explicit exit.

---

### GX-017: $ref system for configuration

**Description:**  
Configuration supports references for reuse:
```yaml
tools:
  - $ref: tool:search  # References registered tool
  - $ref: agent:helper # References registered agent
```

Good for modular configuration.

---

### GX-018: Multi-skill assistant pattern

**Description:**  
MatchAgent enables router pattern:
```
Router (Match) → Weather Agent (ReAct)
              → Music Agent (ReAct)
              → Chat Agent
```

Well-documented in agent/doc.go.

---

### GX-025: Language idioms differ (Go vs Rust)

**Description:**  
Go uses `iter.Seq`, Rust uses iterators/streams. This is idiomatic for each language
and not a functional issue.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| GX-001 | 🔴 Major | Open | Rust |
| GX-002 | 🔴 Major | Open | Rust |
| GX-003 | 🟡 Minor | Open | Go |
| GX-004 | 🟡 Minor | Note | Go |
| GX-006 | 🟡 Minor | Open | Rust |
| GX-019 | 🟡 Minor | Open | Go |
| GX-020 | 🟡 Minor | Open | Go |
| GX-021 | 🟡 Minor | Open | Go |
| GX-022 | 🟡 Minor | Open | Go |
| GX-023 | 🟡 Minor | Open | Go |
| GX-007 | 🔵 Enhancement | Open | Both |
| GX-008 | 🔵 Enhancement | Open | Both |
| GX-009 | 🔵 Enhancement | Open | Both |
| GX-010 | 🔵 Enhancement | Open | Go |
| GX-011 | 🔵 Enhancement | Open | Go |
| GX-012 | 🔵 Enhancement | Open | Go |
| GX-013 | 🔵 Enhancement | Open | Rust |
| GX-024 | 🔵 Enhancement | Open | Rust |
| GX-014 | ⚪ Note | N/A | Go |
| GX-015 | ⚪ Note | N/A | Go |
| GX-016 | ⚪ Note | N/A | Go |
| GX-017 | ⚪ Note | N/A | Go |
| GX-018 | ⚪ Note | N/A | Go |
| GX-025 | ⚪ Note | N/A | Both |

**Overall:** Go implementation is mature and feature-rich with comprehensive agent framework. Rust implementation provides basic LLM abstraction but lacks the agent framework, making it suitable only for simple use cases. Major effort needed to reach Rust feature parity.
