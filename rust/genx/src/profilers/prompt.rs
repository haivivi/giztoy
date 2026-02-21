//! LLM prompt construction for profilers.

use std::collections::HashMap;

use crate::segmentors::types::format_schema_section;
use crate::segmentors::{Schema, SegmentorResult};

use super::types::ProfilerInput;

pub fn build_prompt(input: &ProfilerInput) -> String {
    let mut sb = String::new();
    sb.push_str(PROMPT_BASE);

    if let Some(schema) = &input.schema {
        if !schema.entity_types.is_empty() {
            sb.push_str("\n\n");
            sb.push_str(&build_schema_section(schema));
        }
    }

    if let Some(profiles) = &input.profiles {
        if !profiles.is_empty() {
            sb.push_str("\n\n");
            sb.push_str(&build_profiles_section(profiles));
        }
    }

    sb.push_str("\n\n");
    sb.push_str(&build_extracted_section(&input.extracted));

    sb.push_str("\n\n");
    sb.push_str(PROMPT_OUTPUT_FORMAT);
    sb
}

pub fn build_conversation_text(messages: &[String]) -> String {
    messages.join("\n")
}

fn build_schema_section(schema: &Schema) -> String {
    format_schema_section(
        schema,
        "Current Entity Schema",
        "These are the currently defined entity types and their attributes.\nYou may propose new fields or modifications.",
    )
}

fn build_profiles_section(
    profiles: &HashMap<String, HashMap<String, serde_json::Value>>,
) -> String {
    let mut sb = String::new();
    sb.push_str("## Existing Entity Profiles\n\n");
    sb.push_str(
        "These are the current profiles. Update them with new information from the conversation.\n\n",
    );

    let mut labels: Vec<_> = profiles.keys().collect();
    labels.sort();

    for label in labels {
        let attrs = &profiles[label];
        sb.push_str(&format!("### {}\n", label));
        if attrs.is_empty() {
            sb.push_str("(no attributes yet)\n");
        } else {
            let sorted: std::collections::BTreeMap<_, _> = attrs.iter().collect();
            if let Ok(json) = serde_json::to_string_pretty(&sorted) {
                sb.push_str(&json);
                sb.push('\n');
            }
        }
        sb.push('\n');
    }
    sb
}

fn build_extracted_section(extracted: &SegmentorResult) -> String {
    let mut sb = String::new();
    sb.push_str("## Extracted Metadata (from segmentor)\n\n");
    sb.push_str(
        "The segmentor has already extracted the following. Use this as the basis for profile updates.\n\n",
    );

    sb.push_str("### Segment Summary\n");
    sb.push_str(&extracted.segment.summary);
    sb.push_str("\n\n");

    if !extracted.entities.is_empty() {
        sb.push_str("### Entities\n");
        for e in &extracted.entities {
            sb.push_str(&format!("- {}", e.label));
            if !e.attrs.is_empty() {
                let sorted: std::collections::BTreeMap<_, _> = e.attrs.iter().collect();
                if let Ok(json) = serde_json::to_string(&sorted) {
                    sb.push_str(&format!(": {}", json));
                }
            }
            sb.push('\n');
        }
        sb.push('\n');
    }

    if !extracted.relations.is_empty() {
        sb.push_str("### Relations\n");
        for r in &extracted.relations {
            sb.push_str(&format!("- {} -[{}]-> {}\n", r.from, r.rel_type, r.to));
        }
        sb.push('\n');
    }

    sb
}

const PROMPT_BASE: &str = r#"You are an entity profile analyst. Your task is to update entity profiles and evolve the entity schema based on conversation content and previously extracted metadata.

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
- Match the language of the conversation (if Chinese, write Chinese descriptions)."#;

const PROMPT_OUTPUT_FORMAT: &str = r#"## Output

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
}"#;

#[cfg(test)]
mod tests {
    use super::*;
    use crate::segmentors::{EntityOutput, RelationOutput, SegmentOutput, SegmentorResult};

    fn make_extracted() -> SegmentorResult {
        SegmentorResult {
            segment: SegmentOutput {
                summary: "小明聊了恐龙".into(),
                keywords: vec!["恐龙".into()],
                labels: vec!["person:小明".into()],
            },
            entities: vec![EntityOutput {
                label: "person:小明".into(),
                attrs: HashMap::new(),
            }],
            relations: vec![RelationOutput {
                from: "person:小明".into(),
                to: "topic:恐龙".into(),
                rel_type: "likes".into(),
            }],
        }
    }

    #[test]
    fn t12_3_prompt_contains_key_sections() {
        let input = ProfilerInput {
            messages: vec!["user: hello".into()],
            extracted: make_extracted(),
            schema: None,
            profiles: None,
        };
        let prompt = build_prompt(&input);
        assert!(prompt.contains("entity profile analyst"));
        assert!(prompt.contains("## Extracted Metadata"));
        assert!(prompt.contains("小明聊了恐龙"));
        assert!(prompt.contains("## Output"));
    }

    #[test]
    fn t12_prompt_base_exact_match() {
        assert!(PROMPT_BASE.starts_with("You are an entity profile analyst."));
        assert!(PROMPT_BASE.contains("## Instructions"));
        assert!(PROMPT_BASE.contains("## Rules"));
        assert!(PROMPT_BASE.ends_with("write Chinese descriptions)."));
    }

    #[test]
    fn t12_prompt_output_format_exact_match() {
        assert!(PROMPT_OUTPUT_FORMAT.starts_with("## Output"));
        assert!(PROMPT_OUTPUT_FORMAT.contains("schema_changes"));
        assert!(PROMPT_OUTPUT_FORMAT.contains("profile_updates"));
        assert!(PROMPT_OUTPUT_FORMAT.contains("relations"));
        assert!(PROMPT_OUTPUT_FORMAT.contains("阳光幼儿园"));
    }

    fn testdata_path(rel: &str) -> Option<std::path::PathBuf> {
        let cargo_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
        let path = cargo_dir.join("../../testdata/genx").join(rel);
        if path.exists() { Some(path) } else { None }
    }

    #[test]
    fn t12_prompt_golden_file() {
        let Some(input_path) = testdata_path("profilers/input.json") else { return };
        let golden_path = input_path.parent().unwrap().join("expected_prompt.txt");

        let data = std::fs::read_to_string(&input_path).unwrap();
        let input: ProfilerInput = serde_json::from_str(&data).unwrap();
        let prompt = build_prompt(&input);

        if golden_path.exists() {
            let expected = std::fs::read_to_string(&golden_path).unwrap();
            assert_eq!(prompt, expected, "prompt does not match golden file");
        } else {
            std::fs::write(&golden_path, &prompt).unwrap();
        }
    }

    #[test]
    fn t12_prompt_with_schema_and_profiles() {
        let mut entity_types = HashMap::new();
        entity_types.insert(
            "person".into(),
            crate::segmentors::EntitySchema {
                desc: "A person".into(),
                attrs: HashMap::new(),
            },
        );
        let mut profiles = HashMap::new();
        let mut attrs = HashMap::new();
        attrs.insert("age".into(), serde_json::json!(5));
        profiles.insert("person:小明".into(), attrs);

        let input = ProfilerInput {
            messages: vec![],
            extracted: make_extracted(),
            schema: Some(Schema { entity_types }),
            profiles: Some(profiles),
        };
        let prompt = build_prompt(&input);
        assert!(prompt.contains("## Current Entity Schema"));
        assert!(prompt.contains("## Existing Entity Profiles"));
    }
}
