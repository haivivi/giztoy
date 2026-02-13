package profilers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

// buildPrompt constructs the system prompt for entity profile analysis.
func buildPrompt(input Input) string {
	var sb strings.Builder
	sb.WriteString(promptBase)

	// Include current schema if available.
	if input.Schema != nil && len(input.Schema.EntityTypes) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(buildSchemaSection(input.Schema))
	}

	// Include existing profiles if available.
	if len(input.Profiles) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(buildProfilesSection(input.Profiles))
	}

	// Include extracted metadata from segmentor.
	if input.Extracted != nil {
		sb.WriteString("\n\n")
		sb.WriteString(buildExtractedSection(input.Extracted))
	}

	sb.WriteString("\n\n")
	sb.WriteString(promptOutputFormat)
	return sb.String()
}

// buildConversationText joins messages into a single block.
func buildConversationText(messages []string) string {
	return strings.Join(messages, "\n")
}

// buildSchemaSection describes the current entity schema.
func buildSchemaSection(schema *segmentors.Schema) string {
	var sb strings.Builder
	sb.WriteString("## Current Entity Schema\n\n")
	sb.WriteString("These are the currently defined entity types and their attributes.\n")
	sb.WriteString("You may propose new fields or modifications.\n\n")

	for prefix, es := range schema.EntityTypes {
		fmt.Fprintf(&sb, "### %s\n", prefix)
		if es.Desc != "" {
			fmt.Fprintf(&sb, "%s\n", es.Desc)
		}
		if len(es.Attrs) > 0 {
			sb.WriteString("Attributes:\n")
			for name, attr := range es.Attrs {
				fmt.Fprintf(&sb, "- `%s` (%s): %s\n", name, attr.Type, attr.Desc)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildProfilesSection describes existing entity profiles.
func buildProfilesSection(profiles map[string]map[string]any) string {
	var sb strings.Builder
	sb.WriteString("## Existing Entity Profiles\n\n")
	sb.WriteString("These are the current profiles. Update them with new information from the conversation.\n\n")

	for label, attrs := range profiles {
		fmt.Fprintf(&sb, "### %s\n", label)
		if len(attrs) == 0 {
			sb.WriteString("(no attributes yet)\n")
		} else {
			b, err := json.MarshalIndent(attrs, "", "  ")
			if err == nil {
				sb.Write(b)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildExtractedSection describes the segmentor's output.
func buildExtractedSection(extracted *segmentors.Result) string {
	var sb strings.Builder
	sb.WriteString("## Extracted Metadata (from segmentor)\n\n")
	sb.WriteString("The segmentor has already extracted the following. Use this as the basis for profile updates.\n\n")

	sb.WriteString("### Segment Summary\n")
	sb.WriteString(extracted.Segment.Summary)
	sb.WriteString("\n\n")

	if len(extracted.Entities) > 0 {
		sb.WriteString("### Entities\n")
		for _, e := range extracted.Entities {
			fmt.Fprintf(&sb, "- %s", e.Label)
			if len(e.Attrs) > 0 {
				b, err := json.Marshal(e.Attrs)
				if err == nil {
					fmt.Fprintf(&sb, ": %s", string(b))
				}
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(extracted.Relations) > 0 {
		sb.WriteString("### Relations\n")
		for _, r := range extracted.Relations {
			fmt.Fprintf(&sb, "- %s -[%s]-> %s\n", r.From, r.RelType, r.To)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

const promptBase = `You are an entity profile analyst. Your task is to update entity profiles and evolve the entity schema based on conversation content and previously extracted metadata.

## Instructions

1. Review the original conversation and the segmentor's extracted metadata.
2. For each entity, determine what profile attributes should be updated based on this conversation.
   - Only update attributes that have new or changed information.
   - Preserve existing profile values unless the conversation explicitly contradicts them.
3. If you discover attributes that don't fit the current schema, propose schema changes.
   - Only propose "add" for genuinely new and useful fields.
   - Only propose "modify" if the type or description needs updating.
4. If you discover additional relations not captured by the segmentor, include them.

## Rules

- Profile updates should be factual observations from this conversation.
- Schema changes should be conservative — only propose fields that are likely to be useful across multiple conversations.
- Entity labels must match those from the extracted metadata (format: "type:name").
- Match the language of the conversation (if Chinese, write Chinese descriptions).`

const promptOutputFormat = `## Output

Call the provided function with the analysis result. The argument must be a JSON object with this structure:

{
  "schema_changes": [
    {
      "entity_type": "person",
      "field": "school",
      "def": {"type": "string", "desc": "学校名称"},
      "action": "add"
    }
  ],
  "profile_updates": {
    "person:小明": {
      "age": 5,
      "favorite_dinosaur": "霸王龙",
      "school": "阳光幼儿园"
    }
  },
  "relations": [
    {
      "from": "person:小明",
      "to": "place:阳光幼儿园",
      "rel_type": "attends"
    }
  ]
}`
