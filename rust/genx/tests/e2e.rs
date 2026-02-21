//! E2E tests calling real DashScope API.
//! Run with: DASHSCOPE_API_KEY=... cargo test -p giztoy-genx --test e2e -- --ignored

use std::path::Path;

use giztoy_genx::modelloader::{load_from_dir, MuxSet};
use giztoy_genx::segmentors::{Schema, SegmentorInput, SegmentorResult};
use giztoy_genx::profilers::ProfilerInput;
use giztoy_genx::{collect_text, Generator, ModelContextBuilder};

fn testdata(rel: &str) -> std::path::PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../testdata/genx")
        .join(rel)
}

fn has_dashscope_key() -> bool {
    std::env::var("DASHSCOPE_API_KEY")
        .map(|k| !k.is_empty())
        .unwrap_or(false)
}

fn load_muxes() -> Option<MuxSet> {
    if !has_dashscope_key() {
        return None;
    }
    let models_dir = testdata("segmentors/models");
    if !models_dir.is_dir() {
        return None;
    }
    let mut muxes = MuxSet::new();
    match load_from_dir(&models_dir, &mut muxes) {
        Ok(_) => Some(muxes),
        Err(e) => {
            eprintln!("load_from_dir failed: {}", e);
            None
        }
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
#[ignore]
async fn e2e_generator_dashscope() {
    let muxes = load_muxes().expect("DASHSCOPE_API_KEY required");

    let mut mcb = ModelContextBuilder::new();
    mcb.prompt_text("system", "你是一个助手。用一句话回答。");
    mcb.user_text("user", "你好");
    let ctx = mcb.build();

    let mut stream = muxes
        .generators
        .read()
        .unwrap()
        .generate_stream("qwen/turbo", &ctx)
        .await
        .expect("generate_stream failed");

    let text = tokio::time::timeout(
        std::time::Duration::from_secs(30),
        collect_text(&mut *stream),
    )
    .await
    .expect("generator timed out after 30s")
    .expect("collect_text failed");

    assert!(!text.is_empty(), "response should not be empty");
    eprintln!("generator response: {}", text);
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
#[ignore]
async fn e2e_segmentor_dashscope() {
    let muxes = load_muxes().expect("DASHSCOPE_API_KEY required");

    let conversation = std::fs::read_to_string(testdata("segmentors/conversation_family.txt"))
        .expect("read conversation_family.txt");
    let messages: Vec<String> = conversation.lines().map(String::from).collect();

    let schema_yaml = std::fs::read_to_string(testdata("segmentors/schema_family.yaml"))
        .expect("read schema_family.yaml");
    let schema: Schema = serde_yaml::from_str(&schema_yaml).expect("parse schema");

    let input = SegmentorInput {
        messages,
        schema: Some(schema),
    };

    let result = muxes
        .segmentors
        .process("seg/qwen-turbo", input)
        .await
        .expect("segmentor process failed");

    assert!(!result.segment.summary.is_empty());
    assert!(result.segment.keywords.len() >= 2);
    assert!(result.entities.len() >= 2);
    assert!(result.entities.iter().any(|e| e.label.contains("小明")));
    assert!(result.relations.len() >= 1);
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
#[ignore]
async fn e2e_profiler_dashscope() {
    let muxes = load_muxes().expect("DASHSCOPE_API_KEY required");

    let conversation = std::fs::read_to_string(testdata("segmentors/conversation_family.txt"))
        .expect("read conversation");
    let messages: Vec<String> = conversation.lines().map(String::from).collect();

    let extracted_json = std::fs::read_to_string(testdata("profilers/extracted_family.json"))
        .expect("read extracted_family.json");
    let extracted: SegmentorResult =
        serde_json::from_str(&extracted_json).expect("parse extracted");

    let schema_yaml = std::fs::read_to_string(testdata("segmentors/schema_family.yaml"))
        .expect("read schema");
    let schema: Schema = serde_yaml::from_str(&schema_yaml).expect("parse schema");

    let profiles_json = std::fs::read_to_string(testdata("profilers/profiles_existing.json"))
        .expect("read profiles");
    let profiles = serde_json::from_str(&profiles_json).expect("parse profiles");

    let input = ProfilerInput {
        messages,
        extracted,
        schema: Some(schema),
        profiles: Some(profiles),
    };

    let result = muxes
        .profilers
        .process("prof/qwen-turbo", input)
        .await
        .expect("profiler process failed");

    assert!(!result.profile_updates.is_empty());
    assert!(result.profile_updates.contains_key("person:小明"));
}
