use std::collections::{HashMap, HashSet};
use std::path::PathBuf;
use std::sync::Arc;

use anyhow::{Context, Result, bail};
use giztoy_genx::context::ModelContextBuilder;
use giztoy_genx::modelloader::{MuxSet, load_from_dir};
use giztoy_genx::tool::FuncTool;
use giztoy_graph::{Entity, Relation};
use giztoy_memory::{CompressPolicy, Host, HostConfig, RecallQuery, SegmentInput};
use giztoy_recall::bucket_1h;
use openerp_kv::{KVStore, RedbStore};
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
                models = Some(PathBuf::from(
                    it.next().context("missing value for -models")?,
                ));
            }
            "-generator" | "--generator" => {
                generator = it.next().context("missing value for -generator")?;
            }
            "-v" | "--verbose" => verbose = true,
            "-h" | "--help" => {
                println!("Usage: labeler_rust -models <dir> [-generator <pattern>] [-v]");
                std::process::exit(0);
            }
            other => bail!("unknown arg: {other}"),
        }
    }

    Ok(Args {
        models: models.context("-models is required")?,
        generator,
        verbose,
    })
}

fn build_prompt(text: &str, candidates: &[String], top_k: usize) -> String {
    let mut s = String::from("Select relevant labels from candidates for query-time recall.\n");
    s.push_str("Only return labels from candidates. score in [0,1].\n\n");
    s.push_str("Query:\n");
    s.push_str(text);
    s.push_str("\nCandidates:\n");
    for c in candidates {
        s.push_str("- ");
        s.push_str(c);
        s.push('\n');
    }
    s.push_str(&format!("Return at most {top_k} matches."));
    s
}

async fn select_labels(
    muxes: &MuxSet,
    generator: &str,
    query: &str,
    candidates: &[String],
    top_k: usize,
) -> Result<Vec<String>> {
    let tool = FuncTool::new::<SelectArgs>("select_labels", "Select labels from candidates");
    let mut mcb = ModelContextBuilder::new();
    mcb.prompt_text("labeler", build_prompt(query, candidates, top_k));
    let ctx = mcb.build();

    let generator_arc = {
        let guard = muxes.generators.read().unwrap();
        guard.get_arc(generator)?
    };
    let (_, call) = generator_arc.invoke(generator, &ctx, &tool).await?;
    let parsed: SelectArgs = serde_json::from_str(&call.arguments)?;

    let allow: HashSet<&str> = candidates.iter().map(String::as_str).collect();
    let mut out = Vec::new();
    for m in parsed.matches.unwrap_or_default() {
        if !allow.contains(m.label.as_str()) {
            bail!("out-of-candidate label: {}", m.label);
        }
        if let Some(score) = m.score
            && !(0.0..=1.0).contains(&score)
        {
            bail!("invalid score {} for {}", score, m.label);
        }
        out.push(m.label);
        if out.len() >= top_k {
            break;
        }
    }
    Ok(out)
}

fn collect_candidates(mem: &giztoy_memory::Memory) -> Result<Vec<String>> {
    let entities = mem.graph().list_entities("")?;
    Ok(entities.into_iter().map(|e| e.label).collect())
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = parse_args()?;

    let mut muxes = MuxSet::new();
    let _ = load_from_dir(&args.models, &mut muxes)
        .with_context(|| format!("load models from {}", args.models.display()))?;

    let tmp = tempfile::tempdir()?;
    let db_path = tmp.path().join("labeler_e2e.redb");
    let store: Arc<dyn KVStore> = Arc::new(RedbStore::open(&db_path)?);

    let host = Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: None,
        compress_policy: CompressPolicy::disabled(),
        separator: '\x1F',
    })?;
    let mem = host.open("rust-e2e");

    mem.graph().set_entity(&Entity {
        label: "person:小明".to_string(),
        attrs: HashMap::new(),
    })?;
    mem.graph().set_entity(&Entity {
        label: "topic:恐龙".to_string(),
        attrs: HashMap::new(),
    })?;
    mem.graph().set_entity(&Entity {
        label: "place:上海".to_string(),
        attrs: HashMap::new(),
    })?;
    mem.graph().add_relation(&Relation {
        from: "person:小明".to_string(),
        to: "topic:恐龙".to_string(),
        rel_type: "likes".to_string(),
    })?;

    mem.store_segment(
        SegmentInput {
            summary: "和小明聊了恐龙".to_string(),
            keywords: vec!["恐龙".to_string()],
            labels: vec!["person:小明".to_string(), "topic:恐龙".to_string()],
        },
        bucket_1h(),
    )
    .await?;

    let mut passed = 0usize;
    let mut failed = 0usize;

    // case1: with label selection
    let candidates = collect_candidates(&mem)?;
    match select_labels(&muxes, &args.generator, "小明喜欢什么恐龙", &candidates, 3).await {
        Ok(labels) => {
            let result = mem
                .recall(RecallQuery {
                    labels,
                    text: "恐龙".to_string(),
                    hops: 2,
                    limit: 10,
                })
                .await?;
            if result.segments.is_empty() {
                println!("[FAIL] BasicRecallWithLabeler: empty segments");
                failed += 1;
            } else {
                if args.verbose {
                    println!(
                        "[PASS] BasicRecallWithLabeler: {} segments",
                        result.segments.len()
                    );
                } else {
                    println!("[PASS] BasicRecallWithLabeler");
                }
                passed += 1;
            }
        }
        Err(err) => {
            println!("[FAIL] BasicRecallWithLabeler: {err}");
            failed += 1;
        }
    }

    // case2: without label selection (text-only)
    let result = mem
        .recall(RecallQuery {
            labels: vec![],
            text: "恐龙".to_string(),
            hops: 2,
            limit: 10,
        })
        .await?;
    if result.segments.is_empty() {
        println!("[FAIL] RecallWithoutLabeler: empty segments");
        failed += 1;
    } else {
        println!("[PASS] RecallWithoutLabeler");
        passed += 1;
    }

    println!("Results: {passed} passed, {failed} failed");
    if failed > 0 {
        std::process::exit(1);
    }
    Ok(())
}
