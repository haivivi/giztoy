//! LLM prompt construction for segmentors.

use super::types::{format_schema_section, Schema, SegmentorInput};

pub fn build_prompt(input: &SegmentorInput) -> String {
    let mut sb = String::new();
    sb.push_str(PROMPT_BASE);
    if let Some(schema) = &input.schema
        && !schema.entity_types.is_empty() {
            sb.push_str("\n\n");
            sb.push_str(&build_schema_hint(schema));
        }
    sb.push_str("\n\n");
    sb.push_str(PROMPT_OUTPUT_FORMAT);
    sb
}

pub fn build_conversation_text(messages: &[String]) -> String {
    messages.join("\n")
}

fn build_schema_hint(schema: &Schema) -> String {
    format_schema_section(
        schema,
        "Entity Schema Hint",
        "The following entity types and attributes are expected. \
         Use these as guidance, but you may also discover entities and attributes beyond this schema.",
    )
}

const PROMPT_BASE: &str = r#"You are a conversation segmentor. Your task is to compress a conversation into a structured segment, extracting entities and relations.

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
- If the conversation is in Chinese, write the summary in Chinese. Match the language of the conversation."#;

const PROMPT_OUTPUT_FORMAT: &str = r#"## Output

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

Note: attrs is an array of key-value pairs. Values must be strings (numbers should be stringified, e.g. "5" not 5)."#;

#[cfg(test)]
mod tests {
    use super::*;
    use crate::segmentors::types::*;
    use std::collections::HashMap;

    #[test]
    fn t10_1_build_prompt_no_schema() {
        let input = SegmentorInput {
            messages: vec!["user: 你好".into()],
            schema: None,
        };
        let prompt = build_prompt(&input);
        assert!(prompt.contains("conversation segmentor"));
        assert!(prompt.contains("## Output"));
        assert!(!prompt.contains("Entity Schema Hint"));
    }

    #[test]
    fn t10_2_build_prompt_with_schema() {
        let mut entity_types = HashMap::new();
        entity_types.insert(
            "person".into(),
            EntitySchema {
                desc: "A person".into(),
                attrs: {
                    let mut m = HashMap::new();
                    m.insert(
                        "age".into(),
                        AttrDef {
                            type_: "string".into(),
                            desc: "Age".into(),
                        },
                    );
                    m
                },
            },
        );
        let input = SegmentorInput {
            messages: vec!["user: 你好".into()],
            schema: Some(Schema { entity_types }),
        };
        let prompt = build_prompt(&input);
        assert!(prompt.contains("Entity Schema Hint"));
        assert!(prompt.contains("### person"));
        assert!(prompt.contains("`age`"));
    }

    #[test]
    fn t10_3_build_conversation_text() {
        let messages = vec!["user: 你好".to_string(), "assistant: 你好呀".to_string()];
        let text = build_conversation_text(&messages);
        assert_eq!(text, "user: 你好\nassistant: 你好呀");
    }

    fn testdata_path(rel: &str) -> Option<std::path::PathBuf> {
        let cargo_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
        let path = cargo_dir.join("../../testdata/genx").join(rel);
        if path.exists() { Some(path) } else { None }
    }

    #[test]
    fn t10_4_prompt_basic_golden_file() {
        let Some(input_path) = testdata_path("segmentors/input_basic.json") else { return };
        let golden_path = input_path.parent().unwrap().join("expected_prompt_basic.txt");

        let data = std::fs::read_to_string(&input_path).unwrap();
        let input: SegmentorInput = serde_json::from_str(&data).unwrap();
        let prompt = build_prompt(&input);

        if golden_path.exists() {
            let expected = std::fs::read_to_string(&golden_path).unwrap();
            assert_eq!(prompt, expected, "prompt does not match golden file");
        } else {
            // Generate golden file on first run
            std::fs::write(&golden_path, &prompt).unwrap();
        }
    }

    #[test]
    fn t10_4_prompt_schema_golden_file() {
        let Some(input_path) = testdata_path("segmentors/input_with_schema.json") else { return };
        let golden_path = input_path.parent().unwrap().join("expected_prompt_schema.txt");

        let data = std::fs::read_to_string(&input_path).unwrap();
        let input: SegmentorInput = serde_json::from_str(&data).unwrap();
        let prompt = build_prompt(&input);

        if golden_path.exists() {
            let expected = std::fs::read_to_string(&golden_path).unwrap();
            assert_eq!(prompt, expected, "prompt does not match golden file");
        } else {
            std::fs::write(&golden_path, &prompt).unwrap();
        }
    }
}
