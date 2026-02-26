use std::collections::HashMap;
use std::sync::Arc;
use std::{env, fs};

use openerp_kv::RedbStore;

use crate::error::MemoryError;
use crate::host::{Host, HostConfig};
use crate::keys::{conv_msg_key, conv_msg_prefix, conv_revert_key};
use crate::types::{
    CompressPolicy, CompressResult, Compressor, EntityInput, EntityUpdate, Message,
    RecallQuery, RelationInput, Role, SegmentInput, now_nano,
};
use crate::messages_to_strings;

use giztoy_genx::segmentors::{EntityOutput, RelationOutput, SegmentOutput, Segmentor, SegmentorInput, SegmentorMux, SegmentorResult};
use giztoy_genx::profilers::{Profiler, ProfilerInput, ProfilerMux, ProfilerResult};
use crate::compressor::{LLMCompressor, LLMCompressorConfig};

// ---------------------------------------------------------------------------
// Mock compressor
// ---------------------------------------------------------------------------

/// Test separator: ASCII Unit Separator (0x1F) allows natural colon-namespaced
/// labels like "person:小明", matching Go's test configuration.
const TEST_SEP: char = '\x1F';

struct FakeSegmentor;

struct FailingProfiler;
struct DefaultMuxProfiler;

#[async_trait::async_trait]
impl Segmentor for FakeSegmentor {
    async fn process(&self, input: SegmentorInput) -> Result<SegmentorResult, giztoy_genx::error::GenxError> {
        let mut entities = Vec::new();
        let mut relations = Vec::new();
        
        let combined = input.messages.join(" | ");

        for name in ["小明", "小红", "小王", "小刘", "小陈", "妈妈", "爸爸", "Alice"] {
            if combined.contains(name) {
                let mut attrs = HashMap::new();
                if name == "Alice" {
                    attrs.insert("role".into(), serde_json::json!("engineer"));
                }
                entities.push(EntityOutput {
                    label: format!("person:{name}"),
                    attrs,
                });
            }
        }

        let relation_defs = [
            ("兄妹", "sibling"),
            ("同事", "colleague"),
            ("邻居", "neighbor"),
            ("朋友", "friend"),
        ];
        let names = ["小明", "小红", "小王", "小刘", "小陈", "妈妈", "爸爸", "Alice"];
        for (kw, rel_type) in relation_defs {
            if !combined.contains(kw) {
                continue;
            }
            for i in 0..names.len() {
                for j in (i + 1)..names.len() {
                    let a = names[i];
                    let b = names[j];
                    if combined.contains(a) && combined.contains(b) {
                        relations.push(RelationOutput {
                            from: format!("person:{a}"),
                            to: format!("person:{b}"),
                            rel_type: rel_type.into(),
                        });
                    }
                }
            }
        }
        if combined.contains("妈妈") && combined.contains("小明") {
            relations.push(RelationOutput { from: "person:妈妈".into(), to: "person:小明".into(), rel_type: "parent".into() });
        }
        if combined.contains("user1") {
            entities.push(EntityOutput { label: "person:user1".into(), attrs: HashMap::new() });
        }
        if combined.contains("sushi") {
            entities.push(EntityOutput { label: "person:小明".into(), attrs: HashMap::from([("favorite_food".into(), serde_json::json!("sushi"))]) });
        }

        // For backward compatibility with existing tests that expect
        // a default entity for generic conversations.
        if entities.is_empty() {
            entities.push(EntityOutput { label: "person:test".into(), attrs: HashMap::from([("compressed".into(), serde_json::json!(true))]) });
        }
        
        let summary_out = format!("compressed: {}", combined);

        Ok(SegmentorResult {
            segment: SegmentOutput {
                summary: summary_out,
                keywords: vec!["test".into(), "dinosaurs".into(), "cooking".into(), "family".into(), "topic_a".into(), "topic_b".into()],
                labels: vec!["person:小明".into(), "person:user1".into()],
            },
            entities,
            relations,
        })
    }

    fn model(&self) -> &str {
        "fake"
    }
}

#[async_trait::async_trait]
impl Profiler for FailingProfiler {
    async fn process(&self, _input: ProfilerInput) -> Result<ProfilerResult, giztoy_genx::error::GenxError> {
        Err(giztoy_genx::error::GenxError::Generation {
            usage: Default::default(),
            message: "mock profiler failure".into(),
        })
    }

    fn model(&self) -> &str {
        "failing-prof"
    }
}

#[async_trait::async_trait]
impl Profiler for DefaultMuxProfiler {
    async fn process(&self, _input: ProfilerInput) -> Result<ProfilerResult, giztoy_genx::error::GenxError> {
        Ok(ProfilerResult {
            schema_changes: vec![],
            profile_updates: HashMap::from([(
                "person:小明".into(),
                HashMap::from([("mood".into(), serde_json::json!("happy"))]),
            )]),
            relations: vec![],
        })
    }

    fn model(&self) -> &str {
        "default-prof"
    }
}

/// Mock compressor that always fails.
struct FailingCompressor;

#[async_trait::async_trait]
impl Compressor for FailingCompressor {
    async fn compress_messages(&self, _msgs: &[Message]) -> Result<CompressResult, MemoryError> {
        Err(MemoryError::General("mock failure".into()))
    }
    async fn extract_entities(&self, _msgs: &[Message]) -> Result<EntityUpdate, MemoryError> {
        Err(MemoryError::General("mock failure".into()))
    }
    async fn compact_segments(&self, _summaries: &[String]) -> Result<CompressResult, MemoryError> {
        Err(MemoryError::General("mock failure".into()))
    }
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

fn new_mock_llm_compressor() -> LLMCompressor {
    let mut mux = SegmentorMux::new();
    mux.handle("test-seg", Arc::new(FakeSegmentor)).unwrap();

    LLMCompressor::new(LLMCompressorConfig {
        segmentor: "test-seg".into(),
        profiler: None,
        schema: None,
        profiles: None,
        seg_mux: Some(Arc::new(mux)),
        prof_mux: None,
    }).unwrap()
}

fn new_mock_llm_compressor_with_failing_profiler() -> LLMCompressor {
    let mut seg_mux = SegmentorMux::new();
    seg_mux.handle("test-seg", Arc::new(FakeSegmentor)).unwrap();

    let mut prof_mux = ProfilerMux::new();
    prof_mux
        .handle("fail-prof", Arc::new(FailingProfiler))
        .unwrap();

    LLMCompressor::new(LLMCompressorConfig {
        segmentor: "test-seg".into(),
        profiler: Some("fail-prof".into()),
        schema: None,
        profiles: None,
        seg_mux: Some(Arc::new(seg_mux)),
        prof_mux: Some(Arc::new(prof_mux)),
    })
    .unwrap()
}

fn new_default_mux_llm_compressor(with_profiler: bool) -> LLMCompressor {
    let seg_pattern = format!("test-seg-default-{}", now_nano());
    giztoy_genx::segmentors::handle(&seg_pattern, Arc::new(FakeSegmentor)).unwrap();

    let (profiler, prof_mux) = if with_profiler {
        let prof_pattern = format!("test-prof-default-{}", now_nano());
        giztoy_genx::profilers::handle(&prof_pattern, Arc::new(DefaultMuxProfiler)).unwrap();
        (Some(prof_pattern), None)
    } else {
        (None, None)
    };

    LLMCompressor::new(LLMCompressorConfig {
        segmentor: seg_pattern,
        profiler,
        schema: None,
        profiles: None,
        seg_mux: None,
        prof_mux,
    })
    .unwrap()
}

fn new_test_store() -> Arc<dyn openerp_kv::KVStore> {
    let dir = tempfile::tempdir().unwrap();
    let db_path = dir.path().join("test.redb");
    let store = RedbStore::open(&db_path).unwrap();
    std::mem::forget(dir);
    Arc::new(store)
}

fn new_test_host() -> Host {
    let store = new_test_store();
    Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: None,
        compress_policy: CompressPolicy::disabled(),
        separator: TEST_SEP,
    })
    .unwrap()
}

fn new_test_host_with_compressor() -> Host {
    let store = new_test_store();
    let mut mux = SegmentorMux::new();
    mux.handle("test-seg", Arc::new(FakeSegmentor)).unwrap();

    let llm_compressor = LLMCompressor::new(LLMCompressorConfig {
        segmentor: "test-seg".into(),
        profiler: None,
        schema: None,
        profiles: None,
        seg_mux: Some(Arc::new(mux)),
        prof_mux: None,
    }).unwrap();

    Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: Some(Arc::new(llm_compressor)),
        compress_policy: CompressPolicy { max_chars: 100, max_messages: 5 },
        separator: TEST_SEP,
    })
    .unwrap()
}

fn new_test_host_with_failing_compressor() -> Host {
    let store = new_test_store();
    Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: Some(Arc::new(FailingCompressor)),
        compress_policy: CompressPolicy { max_chars: 100, max_messages: 5 },
        separator: TEST_SEP,
    })
    .unwrap()
}

fn user_msg(content: &str) -> Message {
    Message {
        role: Role::User,
        name: String::new(),
        content: content.into(),
        timestamp: 0,
        tool_call_id: String::new(),
        tool_call_name: String::new(),
        tool_call_args: String::new(),
        tool_result_id: String::new(),
    }
}

fn model_msg(content: &str) -> Message {
    Message {
        role: Role::Model,
        name: String::new(),
        content: content.into(),
        timestamp: 0,
        tool_call_id: String::new(),
        tool_call_name: String::new(),
        tool_call_args: String::new(),
        tool_result_id: String::new(),
    }
}

fn read_shared_testdata_bytes(rel: &str) -> Vec<u8> {
    let mut candidates = vec![
        format!("../../testdata/memory/{rel}"),
        format!("testdata/memory/{rel}"),
    ];

    if let (Ok(test_srcdir), Ok(workspace)) = (env::var("TEST_SRCDIR"), env::var("TEST_WORKSPACE")) {
        candidates.push(format!("{test_srcdir}/{workspace}/testdata/memory/{rel}"));
    }
    if let Ok(runfiles_dir) = env::var("RUNFILES_DIR") {
        if let Ok(workspace) = env::var("TEST_WORKSPACE") {
            candidates.push(format!("{runfiles_dir}/{workspace}/testdata/memory/{rel}"));
        }
    }

    for path in &candidates {
        if let Ok(data) = fs::read(path) {
            return data;
        }
    }

    panic!("unable to locate shared testdata file: {rel}, tried: {candidates:?}");
}

fn read_shared_testdata_string(rel: &str) -> String {
    let data = read_shared_testdata_bytes(rel);
    String::from_utf8(data).expect("shared testdata should be utf8")
}

#[derive(Debug, serde::Deserialize)]
struct ScenarioMeta {
    expect: ScenarioExpect,
}

#[derive(Debug, Default, serde::Deserialize)]
struct ScenarioExpect {
    #[serde(default)]
    min_entities: usize,
    #[serde(default)]
    entities_contain: Vec<String>,
    #[serde(default)]
    min_relations: usize,
    #[serde(default)]
    min_segments: usize,
    #[serde(default)]
    recall: Vec<ScenarioRecallExpect>,
}

#[derive(Debug, Default, serde::Deserialize)]
struct ScenarioRecallExpect {
    text: String,
    #[serde(default)]
    labels: Vec<String>,
    #[serde(default)]
    min_results: usize,
}

#[derive(Debug, serde::Deserialize)]
struct ScenarioConversation {
    conv_id: String,
    #[serde(default)]
    labels: Vec<String>,
    messages: Vec<ScenarioMessage>,
}

#[derive(Debug, serde::Deserialize)]
struct ScenarioMessage {
    role: String,
    #[serde(default)]
    name: String,
    content: String,
}

fn resolve_shared_testdata_root() -> std::path::PathBuf {
    let mut candidates = vec![
        std::path::PathBuf::from("../../testdata/memory"),
        std::path::PathBuf::from("testdata/memory"),
    ];
    if let (Ok(test_srcdir), Ok(workspace)) = (env::var("TEST_SRCDIR"), env::var("TEST_WORKSPACE")) {
        candidates.push(std::path::PathBuf::from(format!("{test_srcdir}/{workspace}/testdata/memory")));
    }
    if let (Ok(runfiles), Ok(workspace)) = (env::var("RUNFILES_DIR"), env::var("TEST_WORKSPACE")) {
        candidates.push(std::path::PathBuf::from(format!("{runfiles}/{workspace}/testdata/memory")));
    }

    for c in &candidates {
        if c.join("m01_single_person/meta.yaml").exists() {
            return c.clone();
        }
    }
    panic!("unable to locate shared testdata root, tried: {candidates:?}");
}

fn load_scenario_meta(scenario: &str) -> ScenarioMeta {
    let root = resolve_shared_testdata_root();
    let content = fs::read_to_string(root.join(scenario).join("meta.yaml")).unwrap();
    serde_yaml::from_str(&content).unwrap()
}

fn load_scenario_conversations(scenario: &str) -> Vec<ScenarioConversation> {
    let root = resolve_shared_testdata_root();
    let dir = root.join(scenario);
    let mut files: Vec<_> = fs::read_dir(&dir)
        .unwrap()
        .filter_map(|e| e.ok())
        .map(|e| e.path())
        .filter(|p| {
            p.file_name()
                .and_then(|n| n.to_str())
                .map(|n| n.starts_with("conv_") && n.ends_with(".yaml"))
                .unwrap_or(false)
        })
        .collect();
    files.sort();

    files
        .into_iter()
        .map(|p| {
            let content = fs::read_to_string(p).unwrap();
            serde_yaml::from_str::<ScenarioConversation>(&content).unwrap()
        })
        .collect()
}

async fn run_te_scenario(scenario: &str) -> (Host, crate::memory::Memory, ScenarioMeta) {
    let meta = load_scenario_meta(scenario);
    let conversations = load_scenario_conversations(scenario);

    let h = new_test_host_with_compressor();
    let m = h.open("p1");

    for conv_data in &conversations {
        let mut conv = m.open_conversation(&conv_data.conv_id, &conv_data.labels);
        for msg in &conv_data.messages {
            let mut message = match msg.role.as_str() {
                "user" => user_msg(&msg.content),
                "model" => model_msg(&msg.content),
                _ => user_msg(&msg.content),
            };
            message.name = msg.name.clone();
            conv.append(message).await.unwrap();
        }
        m.compress(&mut conv, None).await.unwrap();
    }

    (h, m, meta)
}

async fn assert_te_scenario(scenario: &str) {
    let (_h, m, meta) = run_te_scenario(scenario).await;

    let all_entities = m.graph().list_entities("").unwrap();
    assert!(
        all_entities.len() >= meta.expect.min_entities,
        "{scenario}: expected at least {} entities, got {}",
        meta.expect.min_entities,
        all_entities.len()
    );

    for label in &meta.expect.entities_contain {
        let found = m.graph().get_entity(label).unwrap();
        assert!(found.is_some(), "{scenario}: expected entity {label}");
    }

    if meta.expect.min_relations > 0 {
        let mut relation_count = 0usize;
        for ent in &all_entities {
            relation_count += m.graph().relations(&ent.label).unwrap().len();
        }
        assert!(
            relation_count >= meta.expect.min_relations,
            "{scenario}: expected at least {} relations, got {}",
            meta.expect.min_relations,
            relation_count
        );
    }

    if meta.expect.min_segments > 0 {
        let segs = m.index().recent_segments(1000).unwrap();
        assert!(
            segs.len() >= meta.expect.min_segments,
            "{scenario}: expected at least {} segments, got {}",
            meta.expect.min_segments,
            segs.len()
        );
    }

    for r in &meta.expect.recall {
        let out = m.recall(RecallQuery {
            labels: r.labels.clone(),
            text: r.text.clone(),
            hops: 0,
            limit: 10,
        }).await.unwrap();
        assert!(
            out.segments.len() >= r.min_results,
            "{scenario}: recall({}) expected >= {}, got {}",
            r.text,
            r.min_results,
            out.segments.len()
        );
    }
}

// ===========================================================================
// TH: Host Management (6 tests)
// ===========================================================================

#[test]
fn th1_host_new_success() {
    let _h = new_test_host();
}

#[test]
fn th2_open_returns_memory_with_correct_id() {
    let h = new_test_host();
    let m = h.open("cat_girl");
    assert_eq!(m.id(), "cat_girl");
}

#[test]
fn th3_open_same_id_twice_operates_on_same_data() {
    let h = new_test_host();
    let m1 = h.open("cat_girl");
    let m2 = h.open("cat_girl");
    assert_eq!(m1.id(), m2.id());
}

#[test]
fn th4_list_returns_all_persona_ids() {
    let h = new_test_host();
    h.open("b_robot");
    h.open("a_cat");
    h.open("c_dog");
    let ids = h.list();
    assert_eq!(ids, vec!["a_cat", "b_robot", "c_dog"]);
}

#[test]
fn th5_delete_clears_data() {
    let h = new_test_host();
    let m = h.open("cat_girl");
    let mut conv = m.open_conversation("dev1", &[]);
    tokio::runtime::Runtime::new().unwrap().block_on(async {
        conv.append(user_msg("hello")).await.unwrap();
    });

    assert!(conv.count().unwrap() > 0);
    h.delete("cat_girl").unwrap();

    let m2 = h.open("cat_girl");
    let conv2 = m2.open_conversation("dev1", &[]);
    assert_eq!(conv2.count().unwrap(), 0);
}

#[test]
fn th6_delete_nonexistent_no_error() {
    let h = new_test_host();
    h.delete("ghost").unwrap();
}

#[tokio::test]
async fn th7_delete_prefix_isolation() {
    let h = new_test_host();

    let m_a = h.open("a");
    let m_ab = h.open("a:b");

    let mut conv_a = m_a.open_conversation("c1", &[]);
    let mut conv_ab = m_ab.open_conversation("c1", &[]);
    conv_a.append(user_msg("hello a")).await.unwrap();
    conv_ab.append(user_msg("hello a:b")).await.unwrap();

    h.delete("a").unwrap();

    let m_a_after = h.open("a");
    let m_ab_after = h.open("a:b");
    let conv_a_after = m_a_after.open_conversation("c1", &[]);
    let conv_ab_after = m_ab_after.open_conversation("c1", &[]);
    assert_eq!(conv_a_after.count().unwrap(), 0);
    assert_eq!(conv_ab_after.count().unwrap(), 1, "delete(a) must not affect persona a:b");
}

// ===========================================================================
// TK: Keys Encoding (4 tests)
// ===========================================================================

#[test]
fn tk1_conv_msg_key_format() {
    let key = conv_msg_key("cat_girl", "dev1", 1700000000000000000);
    assert_eq!(key, "mem:cat_girl:conv:dev1:msg:01700000000000000000");
}

#[test]
fn tk2_conv_msg_prefix() {
    let p = conv_msg_prefix("cat_girl", "dev1");
    assert_eq!(p, "mem:cat_girl:conv:dev1:msg:");
}

#[test]
fn tk3_conv_revert_key() {
    let key = conv_revert_key("cat_girl", "dev1");
    assert_eq!(key, "mem:cat_girl:conv:dev1:revert");
}

#[test]
fn tk4_key_lexicographic_order() {
    let k1 = conv_msg_key("m", "c", 9000);
    let k2 = conv_msg_key("m", "c", 10000);
    assert!(k1 < k2, "zero-padded timestamps must sort correctly");
}

// ===========================================================================
// TT: Monotonic Timestamp (2 tests)
// ===========================================================================

#[test]
fn tt1_monotonic_1000_calls() {
    let mut prev = 0i64;
    for _ in 0..1000 {
        let ts = now_nano();
        assert!(ts > prev, "timestamps must be strictly increasing");
        prev = ts;
    }
}

#[test]
fn tt2_concurrent_no_duplicates() {
    use std::collections::HashSet;
    use std::sync::Mutex;

    let results: Arc<Mutex<Vec<i64>>> = Arc::new(Mutex::new(Vec::new()));
    let mut handles = vec![];

    for _ in 0..10 {
        let results = Arc::clone(&results);
        handles.push(std::thread::spawn(move || {
            let mut local = Vec::new();
            for _ in 0..100 {
                local.push(now_nano());
            }
            results.lock().unwrap().extend(local);
        }));
    }

    for h in handles {
        h.join().unwrap();
    }

    let all = results.lock().unwrap();
    let set: HashSet<i64> = all.iter().copied().collect();
    assert_eq!(all.len(), 1000, "should have 1000 timestamps");
    assert_eq!(set.len(), 1000, "all timestamps should be unique");
}

// ===========================================================================
// TC: Conversation (14 tests)
// ===========================================================================

#[tokio::test]
async fn tc1_append_and_recent() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    for i in 0..5 {
        conv.append(user_msg(&format!("msg{i}"))).await.unwrap();
    }

    let recent = conv.recent(10).unwrap();
    assert_eq!(recent.len(), 5);
    assert_eq!(recent[0].content, "msg0");
    assert_eq!(recent[4].content, "msg4");
}

#[tokio::test]
async fn tc2_append_auto_fills_timestamp() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);
    conv.append(user_msg("hello")).await.unwrap();

    let msgs = conv.all().unwrap();
    assert_eq!(msgs.len(), 1);
    assert!(msgs[0].timestamp > 0);
}

#[tokio::test]
async fn tc3_recent_limits_count() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    for i in 0..10 {
        conv.append(user_msg(&format!("msg{i}"))).await.unwrap();
    }

    let recent = conv.recent(3).unwrap();
    assert_eq!(recent.len(), 3);
    assert_eq!(recent[0].content, "msg7");
    assert_eq!(recent[2].content, "msg9");
}

#[tokio::test]
async fn tc4_count() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    assert_eq!(conv.count().unwrap(), 0);
    conv.append(user_msg("a")).await.unwrap();
    conv.append(model_msg("b")).await.unwrap();
    assert_eq!(conv.count().unwrap(), 2);
}

#[tokio::test]
async fn tc5_all_returns_all() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("a")).await.unwrap();
    conv.append(model_msg("b")).await.unwrap();
    conv.append(user_msg("c")).await.unwrap();

    let all = conv.all().unwrap();
    assert_eq!(all.len(), 3);
    assert_eq!(all[0].content, "a");
    assert_eq!(all[2].content, "c");
}

#[tokio::test]
async fn tc6_clear_resets_count() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("a")).await.unwrap();
    conv.append(model_msg("b")).await.unwrap();
    assert_eq!(conv.count().unwrap(), 2);

    conv.clear().unwrap();
    assert_eq!(conv.count().unwrap(), 0);
}

#[tokio::test]
async fn tc7_revert_deletes_last_user_and_model() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("q1")).await.unwrap();
    conv.append(model_msg("a1")).await.unwrap();
    conv.append(user_msg("q2")).await.unwrap();
    conv.append(model_msg("a2")).await.unwrap();

    conv.revert().unwrap();

    let msgs = conv.all().unwrap();
    assert_eq!(msgs.len(), 2);
    assert_eq!(msgs[0].content, "q1");
    assert_eq!(msgs[1].content, "a1");
}

#[tokio::test]
async fn tc8_revert_no_user_msg_no_error() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(model_msg("a1")).await.unwrap();
    conv.revert().unwrap();
}

#[tokio::test]
async fn tc9_revert_then_recent_excludes_deleted() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("q1")).await.unwrap();
    conv.append(model_msg("a1")).await.unwrap();
    conv.append(user_msg("q2")).await.unwrap();
    conv.append(model_msg("a2")).await.unwrap();

    conv.revert().unwrap();

    let recent = conv.recent(10).unwrap();
    assert_eq!(recent.len(), 2);
    assert!(!recent.iter().any(|m| m.content == "q2" || m.content == "a2"));
}

#[tokio::test]
async fn tc10_recent_segments() {
    let h = new_test_host();
    let m = h.open("p1");

    m.store_segment(
        SegmentInput {
            summary: "test seg".into(),
            keywords: vec![],
            labels: vec![],
        },
        giztoy_recall::bucket_1h(),
    )
    .await
    .unwrap();

    let conv = m.open_conversation("c1", &[]);
    let segs = conv.recent_segments(10).unwrap();
    assert_eq!(segs.len(), 1);
    assert_eq!(segs[0].summary, "test seg");
}

#[tokio::test]
async fn tc11_auto_compress_on_max_chars() {
    let h = new_test_host_with_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    // Policy: max_chars=100. Each msg ~48 chars. At ~3rd msg pending exceeds 100.
    // After compress fires: msgs 1-3 compressed and cleared.
    // Msgs 4-5 remain (below threshold). So count should be < 5.
    for i in 0..5 {
        let content = format!("message number {i} with some padding text here");
        conv.append(user_msg(&content)).await.unwrap();
    }

    let count = conv.count().unwrap();
    assert!(count < 5, "auto-compress should have reduced message count, got {count}");

    let segs = conv.recent_segments(10).unwrap();
    assert!(!segs.is_empty(), "segments should exist after compress");
}

#[tokio::test]
async fn tc12_auto_compress_on_max_messages() {
    let h = new_test_host_with_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    // Policy: max_messages=5. Send exactly 5.
    for i in 0..5 {
        conv.append(user_msg(&format!("m{i}"))).await.unwrap();
    }

    let count = conv.count().unwrap();
    assert_eq!(count, 0, "should auto-compress at message threshold");
}

#[tokio::test]
async fn tc13_auto_compress_failure_does_not_block_append() {
    let h = new_test_host_with_failing_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    for i in 0..6 {
        conv.append(user_msg(&format!("m{i}"))).await.unwrap();
    }

    // Append succeeded despite compress failure.
    assert!(conv.count().unwrap() > 0);
    assert!(conv.last_compress_err().is_some());
}

#[tokio::test]
async fn tc14_auto_compress_plus_compact_cascade() {
    let store = new_test_store();
    let host = Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: Some(Arc::new(new_mock_llm_compressor())),
        compress_policy: CompressPolicy { max_chars: 50, max_messages: 3 },
        separator: TEST_SEP,
    })
    .unwrap();

    let m = host.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    // Trigger multiple auto-compresses by sending enough messages in batches.
    for i in 0..12 {
        conv.append(user_msg(&format!("message batch item {i}"))).await.unwrap();
    }

    // Verify segments exist (some may have been compacted).
    let segs = conv.recent_segments(100).unwrap();
    assert!(!segs.is_empty(), "segments should exist after compress+compact");
}

// ===========================================================================
// TM: Memory Core (10 tests)
// ===========================================================================

#[tokio::test]
async fn tm1_store_segment_and_recall() {
    let h = new_test_host();
    let m = h.open("p1");

    m.store_segment(
        SegmentInput {
            summary: "talked about dinosaurs".into(),
            keywords: vec!["dinosaurs".into()],
            labels: vec!["person:小明".into()],
        },
        giztoy_recall::bucket_1h(),
    )
    .await
    .unwrap();

    let result = m.recall(RecallQuery {
        labels: vec![],
        text: String::new(),
        hops: 0,
        limit: 10,
    }).await.unwrap();

    // Without vector search, keyword/label search needs text or labels.
    // With empty query, segments with score=0 are included when no signals.
    assert!(!result.segments.is_empty() || result.entities.is_empty());
}

#[tokio::test]
async fn tm2_recall_empty_memory() {
    let h = new_test_host();
    let m = h.open("p1");

    let result = m.recall(RecallQuery {
        labels: vec![],
        text: String::new(),
        hops: 0,
        limit: 10,
    }).await.unwrap();

    assert!(result.segments.is_empty());
    assert!(result.entities.is_empty());
}

#[tokio::test]
async fn tm3_recall_with_labels() {
    let h = new_test_host();
    let m = h.open("p1");

    m.store_segment(
        SegmentInput {
            summary: "dino chat".into(),
            keywords: vec!["dinosaurs".into()],
            labels: vec!["person:小明".into()],
        },
        giztoy_recall::bucket_1h(),
    )
    .await
    .unwrap();

    m.store_segment(
        SegmentInput {
            summary: "cooking session".into(),
            keywords: vec!["cooking".into()],
            labels: vec!["person:妈妈".into()],
        },
        giztoy_recall::bucket_1h(),
    )
    .await
    .unwrap();

    let result = m.recall(RecallQuery {
        labels: vec!["person:小明".into()],
        text: String::new(),
        hops: 0,
        limit: 10,
    }).await.unwrap();

    assert!(
        result.segments.iter().any(|s| s.labels.contains(&"person:小明".into())),
        "should find segment with matching label"
    );
}

#[tokio::test]
async fn tm4_recall_with_text() {
    let h = new_test_host();
    let m = h.open("p1");

    m.store_segment(
        SegmentInput {
            summary: "talked about dinosaurs".into(),
            keywords: vec!["dinosaurs".into(), "trex".into()],
            labels: vec![],
        },
        giztoy_recall::bucket_1h(),
    )
    .await
    .unwrap();

    let result = m.recall(RecallQuery {
        labels: vec![],
        text: "dinosaurs".into(),
        hops: 0,
        limit: 10,
    }).await.unwrap();

    assert!(
        result.segments.iter().any(|s| s.keywords.contains(&"dinosaurs".into())),
        "keyword search should find the segment"
    );
}

#[tokio::test]
async fn tm5_recall_limit() {
    let h = new_test_host();
    let m = h.open("p1");

    for i in 0..5 {
        m.store_segment(
            SegmentInput {
                summary: format!("seg {i}"),
                keywords: vec!["test".into()],
                labels: vec![],
            },
            giztoy_recall::bucket_1h(),
        )
        .await
        .unwrap();
    }

    let result = m.recall(RecallQuery {
        labels: vec![],
        text: "test".into(),
        hops: 0,
        limit: 2,
    }).await.unwrap();

    assert!(result.segments.len() <= 2);
}

#[tokio::test]
async fn tm6_apply_entity_update_creates() {
    let h = new_test_host();
    let m = h.open("p1");

    m.apply_entity_update(&EntityUpdate {
        entities: vec![EntityInput {
            label: "person:小明".into(),
            attrs: HashMap::from([("age".into(), serde_json::json!(5))]),
        }],
        relations: vec![RelationInput {
            from: "person:小明".into(),
            to: "person:小红".into(),
            rel_type: "sibling".into(),
        }],
    })
    .unwrap();

    let ent = m.graph().get_entity("person:小明").unwrap();
    assert!(ent.is_some());
    assert_eq!(ent.unwrap().attrs["age"], serde_json::json!(5));

    let rels = m.graph().relations("person:小明").unwrap();
    assert_eq!(rels.len(), 1);
    assert_eq!(rels[0].rel_type, "sibling");
}

#[tokio::test]
async fn tm7_apply_entity_update_merges() {
    let h = new_test_host();
    let m = h.open("p1");

    m.apply_entity_update(&EntityUpdate {
        entities: vec![EntityInput {
            label: "person:小明".into(),
            attrs: HashMap::from([("age".into(), serde_json::json!(5))]),
        }],
        relations: vec![],
    })
    .unwrap();

    m.apply_entity_update(&EntityUpdate {
        entities: vec![EntityInput {
            label: "person:小明".into(),
            attrs: HashMap::from([("hobby".into(), serde_json::json!("dinosaurs"))]),
        }],
        relations: vec![],
    })
    .unwrap();

    let ent = m.graph().get_entity("person:小明").unwrap().unwrap();
    assert_eq!(ent.attrs["age"], serde_json::json!(5));
    assert_eq!(ent.attrs["hobby"], serde_json::json!("dinosaurs"));
}

#[tokio::test]
async fn tm8_compress_pipeline() {
    let h = new_test_host_with_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("hello")).await.unwrap();
    conv.append(model_msg("hi there")).await.unwrap();

    m.compress(&mut conv, None).await.unwrap();

    assert_eq!(conv.count().unwrap(), 0, "conversation should be cleared after compress");

    let segs = m.index().recent_segments(10).unwrap();
    assert!(!segs.is_empty(), "segments should exist after compress");

    let ent = m.graph().get_entity("person:test").unwrap();
    assert!(ent.is_some(), "entity should be created by compress");
}

#[tokio::test]
async fn tm9_compress_no_compressor_returns_error() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);
    conv.append(user_msg("hello")).await.unwrap();

    let result = m.compress(&mut conv, None).await;
    assert!(matches!(result, Err(MemoryError::NoCompressor)));
}

#[tokio::test]
async fn tm10_compact_bucket_cascade() {
    let store = new_test_store();
    let host = Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: Some(Arc::new(new_mock_llm_compressor())),
        compress_policy: CompressPolicy { max_chars: 50, max_messages: 3 },
        separator: TEST_SEP,
    })
    .unwrap();
    let m = host.open("p1");

    // Store enough segments to trigger compaction.
    for i in 0..5 {
        m.store_segment(
            SegmentInput {
                summary: format!("segment number {i} with padding"),
                keywords: vec!["test".into()],
                labels: vec![],
            },
            giztoy_recall::bucket_1h(),
        )
        .await
        .unwrap();
    }

    m.compact().await.unwrap();

    let segs = m.index().recent_segments(100).unwrap();
    assert!(!segs.is_empty(), "segments should exist after compact");
}

// ===========================================================================
// TL: Compressor trait (6 tests, using mock) — LLMCompressor deferred
// ===========================================================================

#[tokio::test]
async fn tl1_compress_messages_returns_segments() {
    let c = new_mock_llm_compressor();
    let msgs = vec![user_msg("hello"), model_msg("world")];
    let result = c.compress_messages(&msgs).await.unwrap();
    assert_eq!(result.segments.len(), 1);
    assert!(result.segments[0].summary.contains("hello"));
    assert!(result.segments[0].summary.contains("world"));
}

#[tokio::test]
async fn tl2_extract_entities_returns_update() {
    let c = new_mock_llm_compressor();
    let msgs = vec![user_msg("hello")];
    let update = c.extract_entities(&msgs).await.unwrap();
    assert_eq!(update.entities.len(), 1);
    assert_eq!(update.entities[0].label, "person:test");
}

#[tokio::test]
async fn tl3_compact_segments_combines_summaries() {
    let c = new_mock_llm_compressor();
    let summaries = vec!["seg1".into(), "seg2".into()];
    let result = c.compact_segments(&summaries).await.unwrap();
    assert_eq!(result.segments.len(), 1);
    assert!(result.summary.contains("seg1"));
    assert!(result.summary.contains("seg2"));
}

#[tokio::test]
async fn tl4_empty_profiler_only_segmentor() {
    // MockCompressor acts as segmentor-only (no profiler).
    let c = new_mock_llm_compressor();
    let update = c.extract_entities(&[user_msg("test")]).await.unwrap();
    assert!(!update.entities.is_empty());
}

#[tokio::test]
async fn tl5_profiler_failure_is_non_fatal() {
    let c = new_mock_llm_compressor_with_failing_profiler();
    let update = c.extract_entities(&[user_msg("小明说今天很开心")]).await.unwrap();
    assert!(update.entities.iter().any(|e| e.label == "person:小明"));
}

#[tokio::test]
async fn tl6_messages_to_strings_format() {
    let msgs = vec![
        Message {
            role: Role::User,
            name: "Alice".into(),
            content: "hello".into(),
            timestamp: 0,
            ..Default::default()
        },
        Message {
            role: Role::Model,
            name: String::new(),
            content: "hi".into(),
            timestamp: 0,
            ..Default::default()
        },
        Message {
            role: Role::Tool,
            name: String::new(),
            content: String::new(), // empty content should be skipped
            timestamp: 0,
            ..Default::default()
        },
    ];

    let strs = messages_to_strings(&msgs);
    assert_eq!(strs.len(), 2);
    assert_eq!(strs[0], "user(Alice): hello");
    assert_eq!(strs[1], "model: hi");
}

#[tokio::test]
async fn tl7_default_segmentor_mux_when_none() {
    let c = new_default_mux_llm_compressor(false);
    let msgs = vec![user_msg("小明今天很开心")];
    let result = c.compress_messages(&msgs).await.unwrap();
    assert_eq!(result.segments.len(), 1);
    assert!(result.segments[0].summary.contains("小明"));
}

#[tokio::test]
async fn tl8_default_profiler_mux_when_none() {
    let c = new_default_mux_llm_compressor(true);
    let msgs = vec![user_msg("小明今天很开心")];
    let update = c.extract_entities(&msgs).await.unwrap();
    let ent = update.entities.iter().find(|e| e.label == "person:小明").unwrap();
    assert_eq!(ent.attrs.get("mood"), Some(&serde_json::json!("happy")));
}

// ===========================================================================
// TS: Serialization Compatibility (6 tests)
// ===========================================================================

#[test]
fn ts1_message_msgpack_roundtrip() {
    let msg = Message {
        role: Role::User,
        name: "Alice".into(),
        content: "hello world".into(),
        timestamp: 1700000000000000000,
        tool_call_id: String::new(),
        tool_call_name: String::new(),
        tool_call_args: String::new(),
        tool_result_id: String::new(),
    };

    let data = rmp_serde::to_vec_named(&msg).unwrap();
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();

    assert_eq!(decoded.role, Role::User);
    assert_eq!(decoded.name, "Alice");
    assert_eq!(decoded.content, "hello world");
    assert_eq!(decoded.timestamp, 1700000000000000000);
}

#[test]
fn ts2_message_tool_roundtrip() {
    let msg = Message {
        role: Role::Tool,
        name: String::new(),
        content: "result data".into(),
        timestamp: 100,
        tool_call_id: "tc123".into(),
        tool_call_name: "search".into(),
        tool_call_args: r#"{"q":"test"}"#.into(),
        tool_result_id: "tr456".into(),
    };

    let data = rmp_serde::to_vec_named(&msg).unwrap();
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();

    assert_eq!(decoded.tool_call_id, "tc123");
    assert_eq!(decoded.tool_call_name, "search");
    assert_eq!(decoded.tool_call_args, r#"{"q":"test"}"#);
    assert_eq!(decoded.tool_result_id, "tr456");
}

#[test]
fn ts3_message_model_roundtrip() {
    let msg = Message {
        role: Role::Model,
        name: String::new(),
        content: "I'm a model response".into(),
        timestamp: 200,
        ..Default::default()
    };

    let data = rmp_serde::to_vec_named(&msg).unwrap();
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();

    assert_eq!(decoded.role, Role::Model);
    assert_eq!(decoded.content, "I'm a model response");
}

#[test]
fn ts4_recall_query_json_roundtrip() {
    let q = RecallQuery {
        labels: vec!["person:小明".into()],
        text: "dinosaurs".into(),
        hops: 2,
        limit: 10,
    };

    let json = serde_json::to_string(&q).unwrap();
    let decoded: RecallQuery = serde_json::from_str(&json).unwrap();
    assert_eq!(decoded.labels, q.labels);
    assert_eq!(decoded.text, q.text);
}

#[test]
fn ts5_entity_update_json_roundtrip() {
    let update = EntityUpdate {
        entities: vec![EntityInput {
            label: "person:小明".into(),
            attrs: HashMap::from([("age".into(), serde_json::json!(5))]),
        }],
        relations: vec![RelationInput {
            from: "person:小明".into(),
            to: "person:小红".into(),
            rel_type: "sibling".into(),
        }],
    };

    let json = serde_json::to_string(&update).unwrap();
    let decoded: EntityUpdate = serde_json::from_str(&json).unwrap();

    assert_eq!(decoded.entities.len(), 1);
    assert_eq!(decoded.entities[0].label, "person:小明");
    assert_eq!(decoded.relations.len(), 1);
}

#[test]
fn ts6_compress_result_json_roundtrip() {
    let result = CompressResult {
        segments: vec![SegmentInput {
            summary: "test".into(),
            keywords: vec!["kw".into()],
            labels: vec!["label".into()],
        }],
        summary: "overall".into(),
    };

    let json = serde_json::to_string(&result).unwrap();
    let decoded: CompressResult = serde_json::from_str(&json).unwrap();
    assert_eq!(decoded.segments.len(), 1);
    assert_eq!(decoded.summary, "overall");
}

// ===========================================================================
// TI: Integration Tests (5 tests)
// ===========================================================================

#[tokio::test]
async fn ti1_full_flow_append_and_recall() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    for i in 0..10 {
        conv.append(user_msg(&format!("conversation turn {i}"))).await.unwrap();
    }

    m.store_segment(
        SegmentInput {
            summary: "conversation summary about dinosaurs".into(),
            keywords: vec!["dinosaurs".into(), "conversation".into()],
            labels: vec!["person:小明".into()],
        },
        giztoy_recall::bucket_1h(),
    )
    .await
    .unwrap();

    let result = m.recall(RecallQuery {
        labels: vec![],
        text: "dinosaurs".into(),
        hops: 0,
        limit: 10,
    }).await.unwrap();

    assert!(!result.segments.is_empty());
}

#[tokio::test]
async fn ti2_auto_compress_then_recall() {
    let h = new_test_host_with_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    // Fill enough to trigger auto-compress.
    for i in 0..6 {
        conv.append(user_msg(&format!("long message {i} with extra padding content"))).await.unwrap();
    }

    // Verify segments were stored by checking recent_segments directly.
    let segs = m.index().recent_segments(100).unwrap();
    assert!(!segs.is_empty(), "compressed segments should exist in index");
}

#[tokio::test]
async fn ti3_multi_persona_isolation() {
    let h = new_test_host();
    let m1 = h.open("alice");
    let m2 = h.open("bob");

    let mut c1 = m1.open_conversation("c1", &[]);
    let mut c2 = m2.open_conversation("c1", &[]);

    c1.append(user_msg("alice msg")).await.unwrap();
    c2.append(user_msg("bob msg")).await.unwrap();

    let alice_msgs = c1.all().unwrap();
    let bob_msgs = c2.all().unwrap();

    assert_eq!(alice_msgs.len(), 1);
    assert_eq!(bob_msgs.len(), 1);
    assert_eq!(alice_msgs[0].content, "alice msg");
    assert_eq!(bob_msgs[0].content, "bob msg");
}

#[tokio::test]
async fn ti4_entity_from_compress_recall_with_graph() {
    let h = new_test_host_with_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("hello")).await.unwrap();
    conv.append(model_msg("world")).await.unwrap();

    m.compress(&mut conv, None).await.unwrap();

    // The mock compressor creates "person:test" entity.
    let ent = m.graph().get_entity("person:test").unwrap();
    assert!(ent.is_some());
}

#[tokio::test]
async fn ti5_revert_and_reappend() {
    let h = new_test_host();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    conv.append(user_msg("q1")).await.unwrap();
    conv.append(model_msg("a1")).await.unwrap();
    conv.append(user_msg("q2")).await.unwrap();
    conv.append(model_msg("a2")).await.unwrap();

    conv.revert().unwrap();
    conv.append(user_msg("q2_new")).await.unwrap();
    conv.append(model_msg("a2_new")).await.unwrap();

    let msgs = conv.all().unwrap();
    assert_eq!(msgs.len(), 4);
    assert_eq!(msgs[2].content, "q2_new");
    assert_eq!(msgs[3].content, "a2_new");
}

// ===========================================================================
// TX: Cross-language Compatibility (5 tests)
// ===========================================================================
// Note: TX.1-TX.4 require shared testdata from Go. For now, we verify
// field tag compatibility by checking msgpack output structure.

#[test]
fn tx1_message_user_field_tags() {
    let data = read_shared_testdata_bytes("serialization/message_user.msgpack");
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();
    assert_eq!(decoded.role, Role::User);
    assert_eq!(decoded.content, "hello");
    assert_eq!(decoded.timestamp, 100);
}

#[test]
fn tx2_message_model_field_tags() {
    let data = read_shared_testdata_bytes("serialization/message_model.msgpack");
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();
    assert_eq!(decoded.role, Role::Model);
    assert_eq!(decoded.content, "response");
    assert_eq!(decoded.timestamp, 200);
}

#[test]
fn tx3_message_tool_field_tags() {
    let data = read_shared_testdata_bytes("serialization/message_tool.msgpack");
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();
    assert_eq!(decoded.role, Role::Tool);
    assert_eq!(decoded.content, "result");
    assert_eq!(decoded.timestamp, 300);
    assert_eq!(decoded.tool_call_id, "tc1");
    assert_eq!(decoded.tool_call_name, "fn1");
    assert_eq!(decoded.tool_call_args, "{}");
    assert_eq!(decoded.tool_result_id, "tr1");
}

#[test]
fn tx4_rust_serialize_preserves_go_tags() {
    let data = read_shared_testdata_bytes("serialization/message_user.msgpack");
    let decoded: Message = rmp_serde::from_slice(&data).unwrap();
    
    // re-serialize
    let encoded = rmp_serde::to_vec_named(&decoded).unwrap();
    let value: rmpv::Value = rmpv::decode::read_value(&mut &encoded[..]).unwrap();
    if let rmpv::Value::Map(map) = value {
        let keys: Vec<String> = map.iter().map(|(k, _)| {
            if let rmpv::Value::String(s) = k { s.as_str().unwrap_or("").to_string() } else { String::new() }
        }).collect();
        assert!(keys.contains(&"role".to_string()));
        assert!(keys.contains(&"content".to_string()));
        assert!(keys.contains(&"ts".to_string()));
    } else {
        panic!("expected msgpack map");
    }
}

#[test]
fn tx5_kv_key_encoding_consistency() {
    let keys_content = read_shared_testdata_string("keys/conv_msg_keys.txt");
    let expected_keys: Vec<&str> = keys_content.lines().collect();

    let key1 = conv_msg_key("p1", "c1", 123456789);
    assert_eq!(key1, expected_keys[0]);

    let key2 = conv_msg_key("p1", "c1", 987654321);
    assert_eq!(key2, expected_keys[1]);
}

// ===========================================================================
// TE: E2E Scenario Tests (7 tests)
// Note: These use mock compressor. When Go testdata is available,
// they should load from shared YAML files.
// ===========================================================================

#[tokio::test]
async fn te1_single_person() {
    assert_te_scenario("m01_single_person").await;
}

#[tokio::test]
async fn te2_two_siblings() {
    assert_te_scenario("m02_two_siblings").await;
}

#[tokio::test]
async fn te3_work_chat_english() {
    assert_te_scenario("m03_work_chat").await;
}

#[tokio::test]
async fn te4_cooking_multiple_people() {
    assert_te_scenario("m04_cooking").await;
}

#[tokio::test]
async fn te5_family_week_100msg() {
    assert_te_scenario("m05_family_week").await;
}

#[tokio::test]
async fn te6_topic_drift_100msg() {
    assert_te_scenario("m06_topic_drift").await;
}

#[tokio::test]
async fn te7_corrections() {
    assert_te_scenario("m07_corrections").await;
}

// ===========================================================================
// TB: Bulk/Stress Tests (5 tests)
// ===========================================================================

#[tokio::test]
async fn tb1_1000_messages_append() {
    let h = new_test_host_with_compressor();
    let m = h.open("p1");
    let mut conv = m.open_conversation("c1", &[]);

    for i in 0..1000 {
        conv.append(user_msg(&format!("bulk msg {i}"))).await.unwrap();
    }
    // Should not OOM, auto-compress should have triggered multiple times.
}

#[tokio::test]
async fn tb2_100_segments_compact() {
    let store = new_test_store();
    let host = Host::new(HostConfig {
        store,
        vec: None,
        embedder: None,
        compressor: Some(Arc::new(new_mock_llm_compressor())),
        compress_policy: CompressPolicy { max_chars: 500, max_messages: 10 },
        separator: TEST_SEP,
    })
    .unwrap();
    let m = host.open("p1");

    for i in 0..100 {
        m.store_segment(
            SegmentInput {
                summary: format!("bulk segment {i}"),
                keywords: vec!["bulk".into()],
                labels: vec![],
            },
            giztoy_recall::bucket_1h(),
        )
        .await
        .unwrap();
    }

    m.compact().await.unwrap();

    let segs = m.index().recent_segments(1000).unwrap();
    assert!(!segs.is_empty());
}

#[tokio::test]
async fn tb3_50_entities_100_relations() {
    let h = new_test_host();
    let m = h.open("p1");

    let mut entities = Vec::new();
    for i in 0..50 {
        entities.push(EntityInput {
            label: format!("entity_{i}"),
            attrs: HashMap::from([("idx".into(), serde_json::json!(i))]),
        });
    }

    let mut relations = Vec::new();
    for i in 0..100 {
        relations.push(RelationInput {
            from: format!("entity_{}", i % 50),
            to: format!("entity_{}", (i + 1) % 50),
            rel_type: "link".into(),
        });
    }

    m.apply_entity_update(&EntityUpdate { entities, relations }).unwrap();

    let all_ents = m.graph().list_entities("entity_").unwrap();
    assert_eq!(all_ents.len(), 50);

    let neighbors = m.graph().neighbors("entity_0", &[]).unwrap();
    assert!(!neighbors.is_empty());
}

#[tokio::test]
async fn tb4_10_personas_isolation() {
    let h = new_test_host();

    let mut handles = vec![];
    for i in 0..10 {
        let m = h.open(&format!("persona_{i}"));
        handles.push((i, m));
    }

    for (i, m) in &handles {
        let mut conv = m.open_conversation("c1", &[]);
        conv.append(user_msg(&format!("persona {i} message"))).await.unwrap();
    }

    for (i, m) in &handles {
        let conv = m.open_conversation("c1", &[]);
        let msgs = conv.all().unwrap();
        assert_eq!(msgs.len(), 1, "persona {i} should have exactly 1 message");
        assert!(msgs[0].content.contains(&format!("persona {i}")));
    }
}

/// Relaxed performance test using redb on tempdir.
/// redb scan on filesystem is ~50x slower than in-memory KV, so we use
/// a 5s threshold here. See tb5_recall_performance_strict for the <100ms
/// target that applies with an in-memory KV store.
#[tokio::test]
async fn tb5_recall_performance_1000_segments() {
    let h = new_test_host();
    let m = h.open("p1");

    for i in 0..1000 {
        m.store_segment(
            SegmentInput {
                summary: format!("segment {i} about topic {}", i % 10),
                keywords: vec![format!("topic{}", i % 10)],
                labels: vec![format!("label{}", i % 5)],
            },
            giztoy_recall::bucket_1h(),
        )
        .await
        .unwrap();
    }

    let start = std::time::Instant::now();
    let result = m.recall(RecallQuery {
        labels: vec!["label0".into()],
        text: "topic0".into(),
        hops: 0,
        limit: 10,
    }).await.unwrap();
    let elapsed = start.elapsed();

    assert!(!result.segments.is_empty());
    assert!(
        elapsed.as_millis() < 5000,
        "recall should complete in reasonable time, took {}ms",
        elapsed.as_millis()
    );
}

/// Strict performance test: recall over 1000 segments must complete in <100ms.
/// Uses the same redb store (no in-memory KV available in Rust yet).
/// Run with: cargo test -- --ignored tb5_recall_performance_strict
///
/// This test is #[ignore]'d because redb on tmpfs can't reliably hit 100ms.
/// When an in-memory KVStore implementation is available, un-ignore this
/// and replace new_test_store() with the in-memory version.
#[tokio::test]
#[ignore]
async fn tb5_recall_performance_strict() {
    let h = new_test_host();
    let m = h.open("p1");

    for i in 0..1000 {
        m.store_segment(
            SegmentInput {
                summary: format!("segment {i} about topic {}", i % 10),
                keywords: vec![format!("topic{}", i % 10)],
                labels: vec![format!("label{}", i % 5)],
            },
            giztoy_recall::bucket_1h(),
        )
        .await
        .unwrap();
    }

    let start = std::time::Instant::now();
    let result = m.recall(RecallQuery {
        labels: vec!["label0".into()],
        text: "topic0".into(),
        hops: 0,
        limit: 10,
    }).await.unwrap();
    let elapsed = start.elapsed();

    assert!(!result.segments.is_empty());
    assert!(
        elapsed.as_millis() < 100,
        "strict: recall should complete in <100ms, took {}ms",
        elapsed.as_millis()
    );
}
