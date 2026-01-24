# GenX Match - Known Issues

## 🔴 Major Issues

### MTH-001: No Rust implementation

**Description:**  
No Rust pattern matching implementation exists.

**Impact:** Cannot use pattern matching in Rust.

---

## 🟡 Minor Issues

### MTH-002: LLM-dependent accuracy

**Description:**  
Match quality depends on LLM capability. May produce unexpected results with weaker models.

**Impact:** Inconsistent matching across different LLMs.

**Suggestion:** Add model-specific prompt tuning.

---

### MTH-003: No offline matching

**Description:**  
All matching requires LLM API call. No local/offline fallback.

**Impact:** Latency and cost for every match operation.

**Suggestion:** Add regex-based fast path for simple patterns.

---

### MTH-004: Pattern syntax not documented

**Description:**  
The `[variable]` syntax in patterns has implicit rules not fully documented.

**Files:**
- `go/pkg/genx/match/rule.go`

---

## 🔵 Enhancements

### MTH-005: Add pattern validation

**Description:**  
No validation that patterns are syntactically correct.

**Suggestion:** Validate `[var]` references match defined vars.

---

### MTH-006: Add confidence scores

**Description:**  
No confidence score for matches.

**Suggestion:** Ask LLM to include confidence in output.

---

### MTH-007: Add multi-intent detection

**Description:**  
Currently matches one intent per input.

**Suggestion:** Support detecting multiple intents in one input.

---

### MTH-008: Add caching

**Description:**  
No caching of match results.

**Suggestion:** Cache identical inputs for performance.

---

### MTH-009: Add negation patterns

**Description:**  
No way to specify "not this pattern".

**Suggestion:** Add `!pattern` syntax for exclusions.

---

## ⚪ Notes

### MTH-010: Streaming output

**Description:**  
Results are streamed as LLM generates them:

```go
for result, err := range matcher.Match(ctx, model, mctx) {
    // Process as they arrive
}
```

Good for responsive UIs.

---

### MTH-011: Variable typing

**Description:**  
Supports typed variables with automatic parsing:
- `string` (default)
- `int`
- `float`
- `bool`

---

### MTH-012: Example-driven learning

**Description:**  
Rules can include examples for better LLM understanding:

```yaml
examples:
  - input: "我想听周杰伦的稻香"
    output: "music: artist=周杰伦, title=稻香"
```

---

### MTH-013: Clean Result API

**Description:**  
Well-designed Result structure:

```go
type Result struct {
    Rule    string
    Args    map[string]Arg
    RawText string
}
```

With `HasValue` flag for optional extraction.

---

## Summary

| ID | Severity | Status | Component |
|----|----------|--------|-----------|
| MTH-001 | 🔴 Major | Open | Rust |
| MTH-002 | 🟡 Minor | Open | Go |
| MTH-003 | 🟡 Minor | Open | Go |
| MTH-004 | 🟡 Minor | Open | Go |
| MTH-005 | 🔵 Enhancement | Open | Go |
| MTH-006 | 🔵 Enhancement | Open | Go |
| MTH-007 | 🔵 Enhancement | Open | Go |
| MTH-008 | 🔵 Enhancement | Open | Go |
| MTH-009 | 🔵 Enhancement | Open | Go |
| MTH-010 | ⚪ Note | N/A | Go |
| MTH-011 | ⚪ Note | N/A | Go |
| MTH-012 | ⚪ Note | N/A | Go |
| MTH-013 | ⚪ Note | N/A | Go |

**Overall:** Useful LLM-based pattern matching. Main limitations are LLM dependency (latency/cost) and Go-only.
