//! Rule-based intent matching using LLM.
//!
//! This module provides a pattern matching system that uses LLM to match user input
//! against predefined rules and extract structured arguments.
//!
//! # Example
//!
//! ```rust,ignore
//! use giztoy_genx::r#match::{Rule, Matcher, CompileOptions};
//!
//! let rules = vec![
//!     Rule {
//!         name: "play_song".to_string(),
//!         vars: [("title".to_string(), Var { label: "song title".to_string(), r#type: "string".to_string() })].into(),
//!         patterns: vec![Pattern { input: "play [title]".to_string(), output: String::new() }],
//!         ..Default::default()
//!     },
//! ];
//!
//! let matcher = Matcher::compile(&rules, CompileOptions::default())?;
//! println!("{}", matcher.system_prompt());
//! ```

mod matcher;
mod rule;
mod template;

pub use matcher::{collect, Arg, CompileOptions, MatchError, MatchResult, Matcher};
pub use rule::{Example, Pattern, Rule, Var};
