//! Rule types for pattern matching.

use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::collections::HashMap;

/// Variable definition for extracting values from user input.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Var {
    /// Short label for the variable, used in pattern expansion.
    /// Must not contain '[' or ']' characters.
    #[serde(default)]
    pub label: String,

    /// Variable type: "string" | "int" | "float" | "bool".
    /// Defaults to "string" if empty.
    #[serde(default, rename = "type")]
    pub var_type: String,
}

/// Pattern is a single input/output pattern example for one rule.
///
/// JSON/YAML supports:
/// - `"play songs"` (string, output auto-generated)
/// - `["play [title]", "play_song: title=[title]"]` (array with explicit output)
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Pattern {
    /// Input pattern, may contain `[varName]` placeholders.
    pub input: String,
    /// Output format. If empty, will be auto-generated from rule name and vars.
    pub output: String,
}

impl Serialize for Pattern {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        if self.output.is_empty() {
            // Emit as scalar string
            serializer.serialize_str(&self.input)
        } else {
            // Emit as [input, output] array
            use serde::ser::SerializeSeq;
            let mut seq = serializer.serialize_seq(Some(2))?;
            seq.serialize_element(&self.input)?;
            seq.serialize_element(&self.output)?;
            seq.end()
        }
    }
}

impl<'de> Deserialize<'de> for Pattern {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        use serde::de::{Error, Visitor};

        struct PatternVisitor;

        impl<'de> Visitor<'de> for PatternVisitor {
            type Value = Pattern;

            fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
                formatter.write_str("a string or [input, output] array")
            }

            fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
            where
                E: Error,
            {
                Ok(Pattern {
                    input: v.to_string(),
                    output: String::new(),
                })
            }

            fn visit_seq<A>(self, mut seq: A) -> Result<Self::Value, A::Error>
            where
                A: serde::de::SeqAccess<'de>,
            {
                let input: String = seq
                    .next_element()?
                    .ok_or_else(|| Error::invalid_length(0, &"1 or 2 elements"))?;

                let output: String = seq.next_element()?.unwrap_or_default();

                // Check no extra elements
                if seq.next_element::<serde::de::IgnoredAny>()?.is_some() {
                    return Err(Error::invalid_length(3, &"1 or 2 elements"));
                }

                Ok(Pattern { input, output })
            }
        }

        deserializer.deserialize_any(PatternVisitor)
    }
}

/// Example is a structured grounding example for the prompt.
///
/// JSON/YAML supports arrays of 1-3 elements:
/// - `["subject"]` (1 element)
/// - `["subject", "user_text"]` (2 elements)
/// - `["subject", "user_text", "output"]` (3 elements)
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Example {
    /// Subject/description of this example.
    pub subject: String,
    /// User input text.
    pub user_text: String,
    /// Expected formatted output.
    pub formatted_to: String,
}

impl Serialize for Example {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        use serde::ser::SerializeSeq;

        // Emit minimal array
        if !self.formatted_to.is_empty() {
            let mut seq = serializer.serialize_seq(Some(3))?;
            seq.serialize_element(&self.subject)?;
            seq.serialize_element(&self.user_text)?;
            seq.serialize_element(&self.formatted_to)?;
            seq.end()
        } else if !self.user_text.is_empty() {
            let mut seq = serializer.serialize_seq(Some(2))?;
            seq.serialize_element(&self.subject)?;
            seq.serialize_element(&self.user_text)?;
            seq.end()
        } else {
            let mut seq = serializer.serialize_seq(Some(1))?;
            seq.serialize_element(&self.subject)?;
            seq.end()
        }
    }
}

impl<'de> Deserialize<'de> for Example {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        use serde::de::{Error, Visitor};

        struct ExampleVisitor;

        impl<'de> Visitor<'de> for ExampleVisitor {
            type Value = Example;

            fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
                formatter.write_str("an array of 1-3 strings")
            }

            fn visit_seq<A>(self, mut seq: A) -> Result<Self::Value, A::Error>
            where
                A: serde::de::SeqAccess<'de>,
            {
                let subject: String = seq
                    .next_element()?
                    .ok_or_else(|| Error::invalid_length(0, &"1-3 elements"))?;

                let user_text: String = seq.next_element()?.unwrap_or_default();
                let formatted_to: String = seq.next_element()?.unwrap_or_default();

                // Check no extra elements
                if seq.next_element::<serde::de::IgnoredAny>()?.is_some() {
                    return Err(Error::invalid_length(4, &"1-3 elements"));
                }

                Ok(Example {
                    subject,
                    user_text,
                    formatted_to,
                })
            }
        }

        deserializer.deserialize_seq(ExampleVisitor)
    }
}

/// Rule is a schema-driven rule definition.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Rule {
    /// Unique name for this rule (e.g., "play_song", "stop_chat").
    pub name: String,

    /// References is a map of unique name -> description.
    /// When merging multiple rules, entries with the same key are deduplicated.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub references: HashMap<String, String>,

    /// Vars is a map of unique name -> variable definition.
    /// The key is the variable name used in patterns (e.g., "title" for [title]).
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub vars: HashMap<String, Var>,

    /// Pattern examples for this rule.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub patterns: Vec<Pattern>,

    /// Grounding examples for the prompt.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub examples: Vec<Example>,
}

impl Rule {
    /// Create a new rule with the given name.
    pub fn new(name: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            ..Default::default()
        }
    }

    /// Parse a rule from JSON bytes.
    pub fn from_json(data: &[u8]) -> Result<Self, serde_json::Error> {
        serde_json::from_slice(data)
    }

    /// Parse a rule from YAML bytes.
    pub fn from_yaml(data: &[u8]) -> Result<Self, serde_yaml::Error> {
        serde_yaml::from_slice(data)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pattern_json_string() {
        let json = r#""play songs""#;
        let pattern: Pattern = serde_json::from_str(json).unwrap();
        assert_eq!(pattern.input, "play songs");
        assert_eq!(pattern.output, "");

        // Round-trip
        let serialized = serde_json::to_string(&pattern).unwrap();
        assert_eq!(serialized, json);
    }

    #[test]
    fn test_pattern_json_array() {
        let json = r#"["play [title]","play_song: title=[title]"]"#;
        let pattern: Pattern = serde_json::from_str(json).unwrap();
        assert_eq!(pattern.input, "play [title]");
        assert_eq!(pattern.output, "play_song: title=[title]");

        // Round-trip
        let serialized = serde_json::to_string(&pattern).unwrap();
        assert_eq!(serialized, json);
    }

    #[test]
    fn test_example_json_1_element() {
        let json = r#"["test subject"]"#;
        let example: Example = serde_json::from_str(json).unwrap();
        assert_eq!(example.subject, "test subject");
        assert_eq!(example.user_text, "");
        assert_eq!(example.formatted_to, "");

        let serialized = serde_json::to_string(&example).unwrap();
        assert_eq!(serialized, json);
    }

    #[test]
    fn test_example_json_2_elements() {
        let json = r#"["subject","user text"]"#;
        let example: Example = serde_json::from_str(json).unwrap();
        assert_eq!(example.subject, "subject");
        assert_eq!(example.user_text, "user text");
        assert_eq!(example.formatted_to, "");

        let serialized = serde_json::to_string(&example).unwrap();
        assert_eq!(serialized, json);
    }

    #[test]
    fn test_example_json_3_elements() {
        let json = r#"["subject","user text","output"]"#;
        let example: Example = serde_json::from_str(json).unwrap();
        assert_eq!(example.subject, "subject");
        assert_eq!(example.user_text, "user text");
        assert_eq!(example.formatted_to, "output");

        let serialized = serde_json::to_string(&example).unwrap();
        assert_eq!(serialized, json);
    }

    #[test]
    fn test_rule_json() {
        let json = r#"{
            "name": "play_song",
            "vars": {
                "title": {"label": "song title", "type": "string"}
            },
            "patterns": [
                "play music",
                ["play [title]", "play_song: title=[title]"]
            ],
            "examples": [
                ["simple example"],
                ["with input", "I want to play music"],
                ["full example", "play Hello", "play_song: title=Hello"]
            ]
        }"#;

        let rule: Rule = serde_json::from_str(json).unwrap();
        assert_eq!(rule.name, "play_song");
        assert_eq!(rule.vars.len(), 1);
        assert_eq!(rule.vars["title"].label, "song title");
        assert_eq!(rule.patterns.len(), 2);
        assert_eq!(rule.examples.len(), 3);
    }

    #[test]
    fn test_var_default() {
        let var = Var::default();
        assert_eq!(var.label, "");
        assert_eq!(var.var_type, "");
    }
}
