//! Template rendering for match prompts.

use minijinja::{context, Environment};

/// Default prompt template (matches Go version).
pub const DEFAULT_TEMPLATE: &str = r#"<task>
You are a helpful assistant, and your task is to format user text to the pattern matched in user text.
</task>

<rules>
- Translate the user text, except for Titles and Names.
- Format user text to the pattern matched in user text.
- One pattern for one line.
- Move forward once to find the next pattern.
- If the user text does not match any pattern, return nothing
- Each matched sentence must contain the core verb and the relevant noun.
</rules>
{%- if references %}

<references>
{%- for key, value in references|items %}
- {{ key }}: {{ value }}
{%- endfor %}
</references>
{%- endif %}

<patterns>
{%- for rule in rules %}
{%- for pattern in rule.patterns %}
- {{ pattern.input }} -> {{ pattern.output }}
{%- endfor %}
{%- endfor %}
</patterns>

<examples>
{%- set case_num = namespace(value=0) %}
{%- for rule in rules %}
{%- for example in rule.examples %}
{%- set case_num.value = case_num.value + 1 %}
## Case {{ case_num.value }}: {{ example.subject }}

### User text:
{{ example.user_text }}

### Formatted to:
{{ example.formatted_to }}

{%- endfor %}
{%- endfor %}
</examples>"#;

/// Data for rendering the prompt template.
#[derive(Debug, Clone, serde::Serialize)]
pub struct PromptData {
    pub references: std::collections::HashMap<String, String>,
    pub rules: Vec<RuleData>,
}

/// Rule data for template rendering.
#[derive(Debug, Clone, serde::Serialize)]
pub struct RuleData {
    pub name: String,
    pub patterns: Vec<PatternData>,
    pub examples: Vec<ExampleData>,
}

/// Pattern data for template rendering.
#[derive(Debug, Clone, serde::Serialize)]
pub struct PatternData {
    pub input: String,
    pub output: String,
}

/// Example data for template rendering.
#[derive(Debug, Clone, serde::Serialize)]
pub struct ExampleData {
    pub subject: String,
    pub user_text: String,
    pub formatted_to: String,
}

/// Render the prompt template with the given data.
pub fn render_prompt(template: &str, data: &PromptData) -> Result<String, minijinja::Error> {
    let mut env = Environment::new();
    env.add_template("prompt", template)?;

    let tmpl = env.get_template("prompt")?;
    tmpl.render(context! {
        references => data.references,
        rules => data.rules,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_render_empty() {
        let data = PromptData {
            references: std::collections::HashMap::new(),
            rules: vec![],
        };

        let result = render_prompt(DEFAULT_TEMPLATE, &data).unwrap();
        assert!(result.contains("<task>"));
        assert!(result.contains("<patterns>"));
        assert!(result.contains("<examples>"));
        // No references section when empty
        assert!(!result.contains("<references>"));
    }

    #[test]
    fn test_render_with_rules() {
        let data = PromptData {
            references: [("music".to_string(), "乐曲指没有歌词的纯音乐作品".to_string())].into(),
            rules: vec![RuleData {
                name: "play_song".to_string(),
                patterns: vec![PatternData {
                    input: "play [song title]".to_string(),
                    output: "play_song: title=[song title]".to_string(),
                }],
                examples: vec![ExampleData {
                    subject: "简单的播放请求".to_string(),
                    user_text: "播放音乐".to_string(),
                    formatted_to: "play_song".to_string(),
                }],
            }],
        };

        let result = render_prompt(DEFAULT_TEMPLATE, &data).unwrap();
        assert!(result.contains("<references>"));
        assert!(result.contains("music: 乐曲指没有歌词的纯音乐作品"));
        assert!(result.contains("play [song title] -> play_song: title=[song title]"));
        assert!(result.contains("## Case 1: 简单的播放请求"));
    }

    #[test]
    fn test_custom_template() {
        let custom = "Rules: {% for rule in rules %}{{ rule.name }} {% endfor %}";
        let data = PromptData {
            references: std::collections::HashMap::new(),
            rules: vec![
                RuleData {
                    name: "rule1".to_string(),
                    patterns: vec![],
                    examples: vec![],
                },
                RuleData {
                    name: "rule2".to_string(),
                    patterns: vec![],
                    examples: vec![],
                },
            ],
        };

        let result = render_prompt(custom, &data).unwrap();
        assert_eq!(result, "Rules: rule1 rule2 ");
    }
}
