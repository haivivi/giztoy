package recall

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// InferConfig controls how [Index.InferLabels] discovers entities in text.
type InferConfig struct {
	// AttrKeys lists entity attribute keys whose string values are checked
	// for matches in the text, in addition to the entity's label name.
	// Common choices: "name", "nickname", "alias".
	//
	// Array-typed attribute values ([]any containing strings) are also
	// supported — each element is checked individually.
	//
	// If empty, only the entity label's display name is matched.
	AttrKeys []string

	// MinNameLen is the minimum character count (in runes) for a name
	// to be eligible for substring matching. Names shorter than this are
	// skipped to avoid false positives (e.g., a single-character entity
	// matching everywhere). Default is 2 if zero.
	MinNameLen int
}

// InferLabels discovers entity labels from the graph that are referenced
// in the given text. It scans all entities and checks if their display
// names (or configured attribute values) appear as substrings in the text.
//
// For labels with a type prefix (e.g., "person:小明"), the name part after
// the colon is matched against the text. For labels without a prefix
// (e.g., "Alice"), the full label is used.
//
// If cfg is nil, default settings are used (no attribute matching,
// MinNameLen=2).
//
// Returns the matching entity labels sorted alphabetically, deduplicated.
func (idx *Index) InferLabels(ctx context.Context, text string, cfg *InferConfig) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	minLen := 2
	var attrKeys []string
	if cfg != nil {
		if cfg.MinNameLen > 0 {
			minLen = cfg.MinNameLen
		}
		attrKeys = cfg.AttrKeys
	}

	matched := make(map[string]struct{})

	for ent, err := range idx.graph.ListEntities(ctx, "") {
		if err != nil {
			return nil, fmt.Errorf("recall: infer labels: %w", err)
		}

		// Check the display name extracted from the label.
		name := displayName(ent.Label)
		if runeLen(name) >= minLen && strings.Contains(text, name) {
			matched[ent.Label] = struct{}{}
			continue
		}

		// Check configured attribute keys for additional name matches.
		if len(attrKeys) > 0 && ent.Attrs != nil {
			if matchAttrs(text, ent.Attrs, attrKeys, minLen) {
				matched[ent.Label] = struct{}{}
			}
		}
	}

	if len(matched) == 0 {
		return nil, nil
	}

	result := make([]string, 0, len(matched))
	for label := range matched {
		result = append(result, label)
	}
	sort.Strings(result)
	return result, nil
}

// displayName extracts the human-readable name from an entity label.
// For typed labels like "person:小明", it returns "小明".
// For plain labels like "Alice", it returns "Alice".
// If the label has multiple colons (e.g., "a:b:c"), the part after the
// first colon is returned ("b:c").
func displayName(label string) string {
	if idx := strings.IndexByte(label, ':'); idx >= 0 && idx < len(label)-1 {
		return label[idx+1:]
	}
	return label
}

// matchAttrs checks if any of the specified attribute values appear in
// the text. Supports string values and []any slices containing strings.
func matchAttrs(text string, attrs map[string]any, keys []string, minLen int) bool {
	for _, key := range keys {
		val, ok := attrs[key]
		if !ok {
			continue
		}

		switch v := val.(type) {
		case string:
			if runeLen(v) >= minLen && strings.Contains(text, v) {
				return true
			}
		case []any:
			for _, elem := range v {
				if s, ok := elem.(string); ok {
					if runeLen(s) >= minLen && strings.Contains(text, s) {
						return true
					}
				}
			}
		}
	}
	return false
}

// runeLen returns the number of runes in s. This is used instead of
// len(s) because CJK characters are multi-byte but should count as
// single characters for minimum name length checks.
func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
