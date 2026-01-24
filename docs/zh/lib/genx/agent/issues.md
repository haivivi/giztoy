# GenX Agent - Known Issues

## 🔴 Major Issues

### AGT-001: No Rust implementation

**Description:**  
The entire agent framework is Go-only. No Rust implementation exists.

**Impact:** Cannot build autonomous agents in Rust.

**Effort:** High - requires porting entire framework.

---

## 🟡 Minor Issues

### AGT-002: Some internal errors use panic

**Description:**  
Some unexpected states trigger panic instead of returning errors.

**Impact:** Can crash applications on edge cases.

**Suggestion:** Convert panics to error returns.

---

### AGT-003: Event loop complexity

**Description:**  
The event loop pattern requires careful handling of all event types.

**Impact:** Easy to miss edge cases in client code.

**Suggestion:** Add helper functions or simplified API.

---

### AGT-004: Tool execution is synchronous

**Description:**  
Tools execute synchronously in the event loop, blocking other processing.

**Impact:** Long-running tools can delay event delivery.

**Suggestion:** Consider async tool execution option.

---

## 🔵 Enhancements

### AGT-005: Add agent persistence

**Description:**  
Agents don't persist state across restarts. State is in-memory only.

**Suggestion:** Add state serialization/deserialization.

---

### AGT-006: Add agent debugging tools

**Description:**  
Limited visibility into agent reasoning and tool selection.

**Suggestion:** Add verbose mode, step-through debugging.

---

### AGT-007: Add rate limiting for tools

**Description:**  
No built-in rate limiting for tool calls.

**Suggestion:** Add configurable rate limiting per tool.

---

### AGT-008: Add tool result caching

**Description:**  
Same tool calls aren't cached, even with identical inputs.

**Suggestion:** Add optional result caching.

---

## ⚪ Notes

### AGT-009: Well-designed event system

**Description:**  
The event-based API provides excellent control:
- Streaming output chunks
- Tool execution visibility
- Clean termination signals

---

### AGT-010: Quit tool pattern

**Description:**  
The quit tool pattern is elegant:
```yaml
tools:
  - $ref: tool:goodbye
    quit: true
```

Allows explicit agent termination.

---

### AGT-011: Multi-agent routing

**Description:**  
MatchAgent enables complex multi-skill architectures:
```
Router → Weather Agent
       → Music Agent  
       → Chat Agent
```

---

### AGT-012: Comprehensive tool types

**Description:**  
Rich tool ecosystem:
- FuncTool (Go functions)
- GeneratorTool (LLM)
- HTTPTool (API calls)
- CompositeTool (pipelines)
- TextProcessorTool

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| AGT-001 | 🔴 Major | Open | Rust |
| AGT-002 | 🟡 Minor | Open | Go |
| AGT-003 | 🟡 Minor | Open | Go |
| AGT-004 | 🟡 Minor | Open | Go |
| AGT-005 | 🔵 Enhancement | Open | Go |
| AGT-006 | 🔵 Enhancement | Open | Go |
| AGT-007 | 🔵 Enhancement | Open | Go |
| AGT-008 | 🔵 Enhancement | Open | Go |
| AGT-009 | ⚪ Note | N/A | Go |
| AGT-010 | ⚪ Note | N/A | Go |
| AGT-011 | ⚪ Note | N/A | Go |
| AGT-012 | ⚪ Note | N/A | Go |

**Overall:** Mature Go implementation with well-designed architecture. Main limitation is Go-only - no Rust support.
