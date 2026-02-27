use std::collections::HashSet;
use std::path::PathBuf;

use anyhow::{Context, Result, bail};
use giztoy_genx::context::ModelContextBuilder;
use giztoy_genx::modelloader::{MuxSet, load_from_dir};
use giztoy_genx::tool::FuncTool;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone)]
struct Args {
    models: PathBuf,
    generator: String,
    verbose: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
struct Match {
    label: String,
    score: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
struct SelectArgs {
    matches: Option<Vec<Match>>,
}

fn parse_args() -> Result<Args> {
    let mut models: Option<PathBuf> = None;
    let mut generator = String::from("qwen/flash");
    let mut verbose = false;

    let mut it = std::env::args().skip(1);
    while let Some(arg) = it.next() {
        match arg.as_str() {
            "-models" | "--models" => {
                let v = it.next().context("missing value for -models")?;
                models = Some(PathBuf::from(v));
            }
            "-generator" | "--generator" => {
                generator = it.next().context("missing value for -generator")?;
            }
            "-v" | "--verbose" => verbose = true,
            "-h" | "--help" => {
                println!("Usage: labelers_rust -models <dir> [-generator <pattern>] [-v]");
                std::process::exit(0);
            }
            other => bail!("unknown arg: {other}"),
        }
    }

    let models = models.context("-models is required")?;
    Ok(Args {
        models,
        generator,
        verbose,
    })
}

fn build_prompt(text: &str, candidates: &[String], top_k: usize) -> String {
    let mut s = String::from("You are a query label selector for memory recall.\n\n");
    s.push_str(
        "Select labels from the provided candidates that are explicitly relevant to the query.\n\n",
    );
    s.push_str("Rules:\n");
    s.push_str("- You MUST choose labels only from candidates.\n");
    s.push_str("- Prefer precision over recall; do not guess unsupported labels.\n");
    s.push_str("- If nothing is relevant, return an empty matches list.\n");
    s.push_str("- score must be in [0, 1].\n\n");
    s.push_str("## Query\n");
    s.push_str(text);
    s.push_str("\n\n## Candidates\n");
    for c in candidates {
        s.push_str("- ");
        s.push_str(c);
        s.push('\n');
    }
    s.push_str("\n## Output\n");
    s.push_str("Call the provided function with JSON arguments.\n");
    s.push_str(&format!(
        "Limit the number of matches to at most {top_k}.\n"
    ));
    s
}

async fn select_labels(
    muxes: &MuxSet,
    generator: &str,
    text: &str,
    candidates: &[String],
    top_k: usize,
) -> Result<Vec<Match>> {
    let tool = FuncTool::new::<SelectArgs>(
        "select_labels",
        "Select relevant labels from candidate labels for recall.",
    );

    let mut mcb = ModelContextBuilder::new();
    mcb.prompt_text("labeler", build_prompt(text, candidates, top_k));
    let ctx = mcb.build();

    let generator_arc = {
        let guard = muxes.generators.read().unwrap();
        guard
            .get_arc(generator)
            .with_context(|| format!("generator not found: {generator}"))?
    };

    let (_, call) = generator_arc.invoke(generator, &ctx, &tool).await?;
    let parsed: SelectArgs = serde_json::from_str(&call.arguments)
        .with_context(|| format!("invalid tool json: {}", call.arguments))?;

    let allow: HashSet<&str> = candidates.iter().map(String::as_str).collect();
    let mut out = Vec::new();
    for m in parsed.matches.unwrap_or_default() {
        if !allow.contains(m.label.as_str()) {
            bail!("model returned out-of-candidate label: {}", m.label);
        }
        if let Some(score) = m.score
            && !(0.0..=1.0).contains(&score)
        {
            bail!("invalid score for {}: {}", m.label, score);
        }
        out.push(m);
        if out.len() >= top_k {
            break;
        }
    }
    Ok(out)
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = parse_args()?;

    let mut muxes = MuxSet::new();
    let names = load_from_dir(&args.models, &mut muxes)
        .with_context(|| format!("load models from {}", args.models.display()))?;

    if args.verbose {
        println!("registered models: {}", names.len());
    }

    let cases = vec![
        (
            "BasicLabelSelection",
            "我昨天和小明聊了恐龙",
            vec!["person:小明", "person:小红", "topic:恐龙", "place:北京"],
            4usize,
        ),
        (
            "TopKLimit",
            "小红今天在上海画恐龙",
            vec!["person:小红", "topic:恐龙", "topic:画画", "place:上海"],
            2usize,
        ),
    ];

    let mut passed = 0usize;
    let mut failed = 0usize;
    for (name, query, cands, top_k) in cases {
        let candidates: Vec<String> = cands.into_iter().map(|s| s.to_string()).collect();
        match select_labels(&muxes, &args.generator, query, &candidates, top_k).await {
            Ok(matches) => {
                if matches.is_empty() {
                    println!("[FAIL] {name}: empty matches");
                    failed += 1;
                    continue;
                }
                if matches.len() > top_k {
                    println!(
                        "[FAIL] {name}: top_k violated: {} > {}",
                        matches.len(),
                        top_k
                    );
                    failed += 1;
                    continue;
                }
                if args.verbose {
                    println!("[PASS] {name}: {matches:?}");
                } else {
                    println!("[PASS] {name}");
                }
                passed += 1;
            }
            Err(err) => {
                println!("[FAIL] {name}: {err}");
                failed += 1;
            }
        }
    }

    println!("Results: {passed} passed, {failed} failed");
    if failed > 0 {
        std::process::exit(1);
    }
    Ok(())
}
