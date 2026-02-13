package segmentors

import (
	"fmt"
	"strings"
)

// buildPrompt constructs the system prompt for conversation segmentation.
// It instructs the LLM to compress conversation messages into a structured
// segment with entity and relation extraction.
func buildPrompt(input Input) string {
	var sb strings.Builder
	sb.WriteString(promptBase)
	if input.Schema != nil && len(input.Schema.EntityTypes) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(buildSchemaHint(input.Schema))
	}
	sb.WriteString("\n\n")
	sb.WriteString(promptOutputFormat)
	return sb.String()
}

// buildConversationText joins input messages into a single conversation block.
func buildConversationText(messages []string) string {
	return strings.Join(messages, "\n")
}

// buildSchemaHint generates a prompt section describing the expected entity schema.
func buildSchemaHint(schema *Schema) string {
	var sb strings.Builder
	sb.WriteString("## Entity Schema Hint\n\n")
	sb.WriteString("The following entity types and attributes are expected. ")
	sb.WriteString("Use these as guidance, but you may also discover entities and attributes beyond this schema.\n\n")

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

const promptBase = `You are a conversation segmentor. Your task is to compress a conversation into a structured segment, extracting entities and relations.

## Instructions

1. Read the conversation carefully.
2. Write a concise summary (1-3 sentences) capturing the key information.
3. Extract keywords for search indexing.
4. Identify all entities mentioned (people, topics, places, objects, etc.).
   - Each entity must have a label in the format "type:name" (e.g., "person:小明", "topic:恐龙", "place:北京").
   - Extract observable attributes for each entity from the conversation.
5. Identify relations between entities (e.g., "person:小明 likes topic:恐龙").
6. The segment's labels field should list all entity labels referenced in this conversation.

## Rules

- Entity labels must use the format "type:name" where type is lowercase (person, topic, place, object, animal, etc.).
- Only extract information that is explicitly stated or strongly implied in the conversation.
- Attributes should be factual observations from this conversation, not assumptions.
- Relations must reference entities that appear in the entities list.
- If the conversation is in Chinese, write the summary in Chinese. Match the language of the conversation.`

const promptOutputFormat = `## Output

Call the provided function with the extraction result. The argument must be a JSON object with this structure:

{
  "segment": {
    "summary": "...",
    "keywords": ["kw1", "kw2"],
    "labels": ["person:小明", "topic:恐龙"]
  },
  "entities": [
    {
      "label": "person:小明",
      "attrs": [
        {"key": "age", "value": "5"},
        {"key": "favorite_dinosaur", "value": "霸王龙"}
      ]
    }
  ],
  "relations": [
    {
      "from": "person:小明",
      "to": "topic:恐龙",
      "rel_type": "likes"
    }
  ]
}

Note: attrs is an array of key-value pairs. Values must be strings (numbers should be stringified, e.g. "5" not 5).`
