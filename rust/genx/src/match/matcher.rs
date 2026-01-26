//! Matcher implementation for rule-based intent matching.

use std::collections::HashMap;
use std::pin::Pin;

use async_stream::stream;
use futures::Stream;
use regex::Regex;
use serde_json::Value;

use super::rule::{Rule, Var};
use super::template::{
    render_prompt, ExampleData, PatternData, PromptData, RuleData, DEFAULT_TEMPLATE,
};
use crate::context::{ModelContextBuilder, Prompt};
use crate::types::Part;
use crate::{Generator, ModelContext};

/// Valid variable types.
const VALID_VAR_TYPES: &[&str] = &["string", "int", "float", "bool"];

/// Options for compiling a Matcher.
#[derive(Debug, Clone, Default)]
pub struct CompileOptions {
    /// Custom template (overrides the default embedded template).
    pub template: Option<String>,
}

impl CompileOptions {
    /// Create new default options.
    pub fn new() -> Self {
        Self::default()
    }

    /// Set a custom template.
    pub fn with_template(mut self, template: impl Into<String>) -> Self {
        self.template = Some(template.into());
        self
    }
}

/// Argument extracted from a match result.
#[derive(Debug, Clone)]
pub struct Arg {
    /// The extracted value, typed according to Var.var_type.
    pub value: Value,

    /// The variable definition from the rule.
    pub var: Var,

    /// Whether a value was successfully extracted.
    pub has_value: bool,
}

/// Result of a single match.
#[derive(Debug, Clone, Default)]
pub struct MatchResult {
    /// The matched rule name. Empty if no rule matched.
    pub rule: String,

    /// Extracted arguments, keyed by variable name.
    pub args: HashMap<String, Arg>,

    /// Original line when no rule matched.
    pub raw_text: String,
}

impl MatchResult {
    /// Check if this result has a matched rule.
    pub fn has_rule(&self) -> bool {
        !self.rule.is_empty()
    }
}

/// Compiled matcher built from rules.
pub struct Matcher {
    /// Rendered system prompt.
    system_prompt: String,

    /// Rule specs: rule name -> var name -> Var.
    specs: HashMap<String, HashMap<String, Var>>,
}

impl Matcher {
    /// Compile rules into a reusable Matcher.
    pub fn compile(rules: &[Rule], opts: CompileOptions) -> Result<Self, MatchError> {
        let template = opts.template.as_deref().unwrap_or(DEFAULT_TEMPLATE);

        let data = build_prompt_data(rules)?;
        let system_prompt = render_prompt(template, &data)
            .map_err(|e| MatchError::Template(e.to_string()))?;

        let mut specs = HashMap::new();
        for rule in rules {
            if specs.contains_key(&rule.name) {
                // Log warning for duplicate rule names
                eprintln!("match: duplicate rule name, skipping: {}", rule.name);
                continue;
            }
            specs.insert(rule.name.clone(), rule.vars.clone());
        }

        Ok(Self {
            system_prompt,
            specs,
        })
    }

    /// Returns the rendered system prompt for debugging.
    pub fn system_prompt(&self) -> &str {
        &self.system_prompt
    }

    /// Execute the matcher against user input and return streaming results.
    ///
    /// It combines the matcher's system prompt with the user's ModelContext,
    /// generates a stream from the model, and parses output lines into MatchResults.
    pub fn match_stream<'a, G: Generator + 'a>(
        &'a self,
        model: &'a str,
        user_ctx: &'a dyn ModelContext,
        generator: &'a G,
    ) -> Pin<Box<dyn Stream<Item = Result<MatchResult, MatchError>> + Send + 'a>> {
        Box::pin(stream! {
            // Build internal context with system prompt
            let mut mcb = ModelContextBuilder::new();
            mcb.add_prompt(Prompt::new("", &self.system_prompt));
            let internal_ctx = mcb.build();

            // Combine: user context then internal prompt
            let combined = CombinedContext {
                contexts: vec![user_ctx, &internal_ctx],
            };

            // Generate stream
            let stream_result = generator.generate_stream(model, &combined).await;
            let mut llm_stream = match stream_result {
                Ok(s) => s,
                Err(e) => {
                    yield Err(MatchError::Generation(e.to_string()));
                    return;
                }
            };

            let mut pending = String::new();

            loop {
                let chunk = llm_stream.next().await;
                match chunk {
                    Ok(Some(message_chunk)) => {
                        // Extract text from chunk
                        if let Some(part) = message_chunk.part {
                            if let Part::Text(text) = part {
                                pending.push_str(&text);
                            }
                        }

                        // Process complete lines
                        while let Some(newline_pos) = pending.find('\n') {
                            let line = pending[..newline_pos].to_string();
                            pending = pending[newline_pos + 1..].to_string();

                            let line = line.trim();
                            if !line.is_empty() {
                                let result = self.parse_line(line);
                                yield Ok(result);
                            }
                        }
                    }
                    Ok(None) => {
                        // Stream ended, process remaining
                        let line = pending.trim();
                        if !line.is_empty() {
                            let result = self.parse_line(line);
                            yield Ok(result);
                        }
                        break;
                    }
                    Err(e) => {
                        // Check if it's ErrDone (normal termination)
                        if e.to_string().contains("done") {
                            // Process remaining
                            let line = pending.trim();
                            if !line.is_empty() {
                                let result = self.parse_line(line);
                                yield Ok(result);
                            }
                        } else {
                            yield Err(MatchError::Generation(e.to_string()));
                        }
                        break;
                    }
                }
            }
        })
    }

    /// Parse a single output line into a MatchResult.
    ///
    /// Format: "rule_name: key1=value1, key2=value2" or just "rule_name"
    /// If the line doesn't match any known rule, returns Result with empty Rule/Args and RawText set.
    pub fn parse_line(&self, line: &str) -> MatchResult {
        let line = line.trim();

        let (name, kv) = match line.split_once(':') {
            Some((n, k)) => (n.trim(), Some(k.trim())),
            None => (line, None),
        };

        if name.is_empty() {
            return MatchResult {
                raw_text: line.to_string(),
                ..Default::default()
            };
        }

        // Check if it's a known rule
        let Some(vars) = self.specs.get(name) else {
            // Not a known rule - return as raw text
            return MatchResult {
                raw_text: line.to_string(),
                ..Default::default()
            };
        };

        // Known rule - parse arguments
        let args = self.parse_kv_to_args(kv.unwrap_or(""), vars);

        MatchResult {
            rule: name.to_string(),
            args,
            raw_text: String::new(),
        }
    }

    /// Parse "key1=value1, key2=value2" into Args using var definitions.
    fn parse_kv_to_args(&self, kv: &str, vars: &HashMap<String, Var>) -> HashMap<String, Arg> {
        let mut args = HashMap::new();

        // Pre-fill all known vars with has_value=false
        for (name, var) in vars {
            args.insert(
                name.clone(),
                Arg {
                    value: Value::Null,
                    var: var.clone(),
                    has_value: false,
                },
            );
        }

        if kv.trim().is_empty() {
            return args;
        }

        for part in kv.split(',') {
            let part = part.trim();
            if part.is_empty() {
                continue;
            }

            let Some((k, v)) = part.split_once('=') else {
                continue;
            };

            let k = k.trim();
            let v = v.trim();
            if k.is_empty() {
                continue;
            }

            let Some(var_def) = vars.get(k) else {
                continue;
            };

            // Convert value based on Var.var_type
            let typed_value = match var_def.var_type.as_str() {
                "int" => v.parse::<i64>().map(Value::from).unwrap_or(Value::String(v.to_string())),
                "float" => v.parse::<f64>().map(Value::from).unwrap_or(Value::String(v.to_string())),
                "bool" => parse_bool(v).map(Value::from).unwrap_or(Value::String(v.to_string())),
                _ => Value::String(v.to_string()), // "string" or empty
            };

            args.insert(
                k.to_string(),
                Arg {
                    value: typed_value,
                    var: var_def.clone(),
                    has_value: true,
                },
            );
        }

        args
    }
}

/// Parse a boolean value from string.
fn parse_bool(s: &str) -> Option<bool> {
    match s.to_lowercase().as_str() {
        "true" | "1" | "yes" | "on" => Some(true),
        "false" | "0" | "no" | "off" => Some(false),
        _ => None,
    }
}

/// Build prompt data from rules.
fn build_prompt_data(rules: &[Rule]) -> Result<PromptData, MatchError> {
    let mut data = PromptData {
        references: HashMap::new(),
        rules: Vec::new(),
    };

    // Regex for placeholder matching: [varName]
    // Only match ASCII word characters to avoid matching Chinese characters in brackets
    let placeholder_re = Regex::new(r"\[([a-zA-Z_][a-zA-Z0-9_]*)\]").unwrap();

    for rule in rules {
        // Validate var types and labels
        for (name, var) in &rule.vars {
            if !var.var_type.is_empty() && !VALID_VAR_TYPES.contains(&var.var_type.as_str()) {
                return Err(MatchError::Validation(format!(
                    "rule {:?}: var {:?} has invalid type {:?} (expected string|int|float|bool)",
                    rule.name, name, var.var_type
                )));
            }
            if var.label.contains('[') || var.label.contains(']') {
                return Err(MatchError::Validation(format!(
                    "rule {:?}: var {:?} label must not contain '[' or ']'",
                    rule.name, name
                )));
            }
        }

        // Validate patterns
        for (i, pattern) in rule.patterns.iter().enumerate() {
            if pattern.input.contains('\n') || pattern.input.contains('\r') {
                return Err(MatchError::Validation(format!(
                    "rule {:?}: pattern[{}] input contains newline",
                    rule.name, i
                )));
            }
            if pattern.output.contains('\n') || pattern.output.contains('\r') {
                return Err(MatchError::Validation(format!(
                    "rule {:?}: pattern[{}] output contains newline",
                    rule.name, i
                )));
            }

            // Placeholders must exist in vars
            for cap in placeholder_re.captures_iter(&pattern.input) {
                let var_name = &cap[1];
                if !rule.vars.contains_key(var_name) {
                    return Err(MatchError::Validation(format!(
                        "rule {:?}: pattern[{}] has placeholder [{}] not defined in vars",
                        rule.name, i, var_name
                    )));
                }
            }
        }

        // Merge references (deduplicate by key)
        data.references.extend(rule.references.clone());

        // Build rule data
        let mut rule_data = RuleData {
            name: rule.name.clone(),
            patterns: Vec::new(),
            examples: rule
                .examples
                .iter()
                .map(|e| ExampleData {
                    subject: e.subject.clone(),
                    user_text: e.user_text.clone(),
                    formatted_to: e.formatted_to.clone(),
                })
                .collect(),
        };

        for pattern in &rule.patterns {
            let (input, output) = if pattern.output.is_empty() {
                expand_pattern(&rule.name, &pattern.input, &rule.vars, &placeholder_re)
            } else {
                (pattern.input.clone(), pattern.output.clone())
            };

            rule_data.patterns.push(PatternData { input, output });
        }

        data.rules.push(rule_data);
    }

    Ok(data)
}

/// Expand [varName] placeholders using var labels and generate output format.
///
/// e.g., "play [title]" with ruleName="play_song" and vars{title: {label: "song title"}} =>
///   input:  "play [song title]"
///   output: "play_song: title=[song title]"
fn expand_pattern(
    rule_name: &str,
    input: &str,
    vars: &HashMap<String, Var>,
    placeholder_re: &Regex,
) -> (String, String) {
    if input.is_empty() {
        return (String::new(), rule_name.to_string());
    }

    let mut out_parts = Vec::new();

    let expanded = placeholder_re
        .replace_all(input, |caps: &regex::Captures| {
            let var_name = &caps[1];
            if let Some(var) = vars.get(var_name) {
                if !var.label.is_empty() {
                    let label = format!("[{}]", var.label);
                    out_parts.push(format!("{}={}", var_name, label));
                    return label;
                }
            }
            // No label defined, keep original
            caps[0].to_string()
        })
        .to_string();

    if out_parts.is_empty() {
        (expanded, rule_name.to_string())
    } else {
        (expanded, format!("{}: {}", rule_name, out_parts.join(", ")))
    }
}

/// Errors that can occur during matching.
#[derive(Debug, thiserror::Error)]
pub enum MatchError {
    #[error("validation error: {0}")]
    Validation(String),

    #[error("template error: {0}")]
    Template(String),

    #[error("generation error: {0}")]
    Generation(String),
}

/// Combined context for multiple ModelContexts.
struct CombinedContext<'a> {
    contexts: Vec<&'a dyn ModelContext>,
}

impl ModelContext for CombinedContext<'_> {
    fn prompts(&self) -> Box<dyn Iterator<Item = &Prompt> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.prompts()))
    }

    fn messages(&self) -> Box<dyn Iterator<Item = &crate::Message> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.messages()))
    }

    fn cots(&self) -> Box<dyn Iterator<Item = &str> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.cots()))
    }

    fn tools(&self) -> Box<dyn Iterator<Item = &dyn crate::Tool> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.tools()))
    }

    fn params(&self) -> Option<&crate::context::ModelParams> {
        self.contexts.iter().find_map(|c| c.params())
    }
}

/// Collect all results from a match stream.
pub async fn collect<S>(mut stream: S) -> Result<Vec<MatchResult>, MatchError>
where
    S: Stream<Item = Result<MatchResult, MatchError>> + Unpin,
{
    use futures::StreamExt;

    let mut results = Vec::new();
    while let Some(item) = stream.next().await {
        results.push(item?);
    }
    Ok(results)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::r#match::Pattern;

    #[test]
    fn test_parse_kv_type_conversion() {
        let vars: HashMap<String, Var> = [
            ("str_var".to_string(), Var { label: "string var".to_string(), var_type: "string".to_string() }),
            ("int_var".to_string(), Var { label: "int var".to_string(), var_type: "int".to_string() }),
            ("float_var".to_string(), Var { label: "float var".to_string(), var_type: "float".to_string() }),
            ("bool_var".to_string(), Var { label: "bool var".to_string(), var_type: "bool".to_string() }),
        ].into();

        let specs: HashMap<String, HashMap<String, Var>> = [
            ("test_rule".to_string(), vars),
        ].into();

        let matcher = Matcher {
            system_prompt: String::new(),
            specs,
        };

        // Test int parsing
        let result = matcher.parse_line("test_rule: int_var=42");
        assert_eq!(result.rule, "test_rule");
        assert_eq!(result.args["int_var"].value, Value::from(42i64));
        assert!(result.args["int_var"].has_value);

        // Test float parsing
        let result = matcher.parse_line("test_rule: float_var=3.14");
        assert_eq!(result.args["float_var"].value, Value::from(3.14f64));

        // Test bool parsing
        let result = matcher.parse_line("test_rule: bool_var=true");
        assert_eq!(result.args["bool_var"].value, Value::from(true));

        // Test string
        let result = matcher.parse_line("test_rule: str_var=hello");
        assert_eq!(result.args["str_var"].value, Value::String("hello".to_string()));
    }

    #[test]
    fn test_parse_line_unknown_rule() {
        let matcher = Matcher {
            system_prompt: String::new(),
            specs: HashMap::new(),
        };

        let result = matcher.parse_line("unknown_rule: x=1");
        assert_eq!(result.rule, "");
        assert_eq!(result.raw_text, "unknown_rule: x=1");
    }

    #[test]
    fn test_parse_line_rule_without_args() {
        let specs: HashMap<String, HashMap<String, Var>> = [
            ("stop".to_string(), HashMap::new()),
        ].into();

        let matcher = Matcher {
            system_prompt: String::new(),
            specs,
        };

        let result = matcher.parse_line("stop");
        assert_eq!(result.rule, "stop");
        assert!(result.args.is_empty());
    }

    #[test]
    fn test_compile_basic() {
        let rules = vec![
            Rule {
                name: "play_song".to_string(),
                vars: [("title".to_string(), Var { label: "song title".to_string(), var_type: "string".to_string() })].into(),
                patterns: vec![Pattern { input: "play [title]".to_string(), output: String::new() }],
                ..Default::default()
            },
            Rule {
                name: "stop".to_string(),
                patterns: vec![Pattern { input: "stop".to_string(), output: String::new() }],
                ..Default::default()
            },
        ];

        let matcher = Matcher::compile(&rules, CompileOptions::default()).unwrap();

        assert!(!matcher.system_prompt().is_empty());
        assert!(matcher.system_prompt().contains("play [song title]"));
        assert!(matcher.system_prompt().contains("play_song: title=[song title]"));
        assert!(matcher.specs.contains_key("play_song"));
        assert!(matcher.specs.contains_key("stop"));
    }

    #[test]
    fn test_compile_validation_errors() {
        // Invalid var type
        let rules = vec![Rule {
            name: "test".to_string(),
            vars: [("x".to_string(), Var { label: "".to_string(), var_type: "invalid".to_string() })].into(),
            ..Default::default()
        }];
        assert!(Matcher::compile(&rules, CompileOptions::default()).is_err());

        // Label contains bracket
        let rules = vec![Rule {
            name: "test".to_string(),
            vars: [("x".to_string(), Var { label: "test[bracket]".to_string(), var_type: "string".to_string() })].into(),
            ..Default::default()
        }];
        assert!(Matcher::compile(&rules, CompileOptions::default()).is_err());

        // Undefined placeholder
        let rules = vec![Rule {
            name: "test".to_string(),
            patterns: vec![Pattern { input: "test [undefined]".to_string(), output: String::new() }],
            ..Default::default()
        }];
        assert!(Matcher::compile(&rules, CompileOptions::default()).is_err());

        // Newline in pattern
        let rules = vec![Rule {
            name: "test".to_string(),
            patterns: vec![Pattern { input: "test\nwith\nnewline".to_string(), output: String::new() }],
            ..Default::default()
        }];
        assert!(Matcher::compile(&rules, CompileOptions::default()).is_err());
    }

    #[test]
    fn test_expand_pattern() {
        let vars: HashMap<String, Var> = [
            ("title".to_string(), Var { label: "song title".to_string(), var_type: "string".to_string() }),
            ("volume".to_string(), Var { label: "volume level".to_string(), var_type: "int".to_string() }),
        ].into();

        let re = Regex::new(r"\[([a-zA-Z_][a-zA-Z0-9_]*)\]").unwrap();

        // Single var
        let (input, output) = expand_pattern("play_song", "play [title]", &vars, &re);
        assert_eq!(input, "play [song title]");
        assert_eq!(output, "play_song: title=[song title]");

        // Multiple vars
        let (input, output) = expand_pattern("set_volume", "play [title] at [volume]", &vars, &re);
        assert_eq!(input, "play [song title] at [volume level]");
        assert!(output.contains("title=[song title]"));
        assert!(output.contains("volume=[volume level]"));

        // No vars
        let (input, output) = expand_pattern("stop", "stop playback", &vars, &re);
        assert_eq!(input, "stop playback");
        assert_eq!(output, "stop");

        // Empty input
        let (input, output) = expand_pattern("empty", "", &vars, &re);
        assert_eq!(input, "");
        assert_eq!(output, "empty");
    }

    #[test]
    fn test_custom_template() {
        let custom = "Custom: {% for rule in rules %}{{ rule.name }} {% endfor %}";
        let rules = vec![Rule::new("test_rule")];

        let matcher = Matcher::compile(&rules, CompileOptions::default().with_template(custom)).unwrap();
        assert_eq!(matcher.system_prompt(), "Custom: test_rule ");
    }
}
