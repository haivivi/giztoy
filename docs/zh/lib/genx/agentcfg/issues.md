# GenX Agent Configuration - Known Issues

## 🔴 Major Issues

### CFG-001: No Rust implementation

**Description:**  
No Rust configuration parser exists.

**Impact:** Cannot define agents in Rust via config files.

---

## 🟡 Minor Issues

### CFG-002: Complex unmarshal logic

**File:** `go/pkg/genx/agentcfg/unmarshal.go`

**Description:**  
The unmarshal logic handles many edge cases and type variations.

**Note:** Well-tested (see `*_unmarshal_test.go` files).

---

### CFG-003: Reference resolution at runtime

**Description:**  
`$ref` references are only validated at runtime, not parse time.

**Impact:** Invalid references fail late.

**Suggestion:** Add config validation command.

---

### CFG-004: No schema generation

**Description:**  
No JSON Schema generation for config files.

**Impact:** No IDE autocompletion support.

**Suggestion:** Generate JSON Schema from types.

---

## 🔵 Enhancements

### CFG-005: Add config inheritance

**Description:**  
No way to extend/inherit agent configs.

**Suggestion:** Add `extends` field:
```yaml
extends: base_agent
name: specialized
```

---

### CFG-006: Add config includes

**Description:**  
No way to include external config files.

**Suggestion:** Add `$include` directive:
```yaml
tools:
  - $include: common_tools.yaml
```

---

### CFG-007: Add environment variable substitution

**Description:**  
No way to use env vars in config values.

**Suggestion:** Support `${ENV_VAR}` syntax.

---

### CFG-008: Add config diff/merge

**Description:**  
No utilities for comparing or merging configs.

---

## ⚪ Notes

### CFG-009: Comprehensive type system

**Description:**  
Rich type system covering:
- Agent types (react, match)
- Tool types (http, generator, composite, text_processor)
- Reference system ($ref)
- Context layers

---

### CFG-010: Multi-format support

**Description:**  
Supports JSON, YAML, and MessagePack serialization.

---

### CFG-011: Extensive test coverage

**Description:**  
Comprehensive test files:
- `agent_marshal_test.go`
- `agent_unmarshal_test.go`
- `tool_marshal_test.go`
- `tool_unmarshal_test.go`
- `testdata/` with JSON/YAML examples

---

### CFG-012: Validation during unmarshal

**Description:**  
Validation happens automatically during JSON/YAML unmarshal.

```go
err := json.Unmarshal(data, &agent)
// Validates required fields, types, etc.
```

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| CFG-001 | 🔴 Major | Open | Rust |
| CFG-002 | 🟡 Minor | Note | Go |
| CFG-003 | 🟡 Minor | Open | Go |
| CFG-004 | 🟡 Minor | Open | Go |
| CFG-005 | 🔵 Enhancement | Open | Go |
| CFG-006 | 🔵 Enhancement | Open | Go |
| CFG-007 | 🔵 Enhancement | Open | Go |
| CFG-008 | 🔵 Enhancement | Open | Go |
| CFG-009 | ⚪ Note | N/A | Go |
| CFG-010 | ⚪ Note | N/A | Go |
| CFG-011 | ⚪ Note | N/A | Go |
| CFG-012 | ⚪ Note | N/A | Go |

**Overall:** Well-designed configuration system with comprehensive type coverage. Main limitation is Go-only.
