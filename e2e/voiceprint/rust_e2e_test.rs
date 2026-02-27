use giztoy_voiceprint::{Detector, DetectorConfig, Hasher};
use serde_json::json;

#[test]
fn t_e2e_voiceprint_rust_pipeline() {
    let expected = json!({
        "hash_a": "82A9",
        "warmup_status": "single",
        "final_status": "single",
        "label_changed": true,
    });

    let hasher = Hasher::default_512();
    let emb_a: Vec<f32> = (0..512).map(|i| i as f32 * 0.01).collect();
    let emb_b: Vec<f32> = (0..512).map(|i| -(i as f32) * 0.01).collect();

    let hash_a = hasher.hash(&emb_a);
    let hash_b = hasher.hash(&emb_b);
    assert_ne!(
        hash_a, hash_b,
        "two speakers should map to different hashes"
    );

    let mut detector = Detector::with_config(DetectorConfig {
        window_size: 5,
        min_ratio: 0.6,
    });

    let mut warmup = None;
    for _ in 0..5 {
        warmup = detector.feed(&hash_a);
    }
    let warmup = warmup.expect("warmup chunk should exist");
    assert_eq!(warmup.status.to_string(), "single");

    let mut final_chunk = None;
    for _ in 0..5 {
        final_chunk = detector.feed(&hash_b);
    }
    let final_chunk = final_chunk.expect("final chunk should exist");

    let got = json!({
        "hash_a": hash_a,
        "warmup_status": warmup.status.to_string(),
        "final_status": final_chunk.status.to_string(),
        "label_changed": warmup.speaker != final_chunk.speaker,
    });

    assert_eq!(got, expected);
}
