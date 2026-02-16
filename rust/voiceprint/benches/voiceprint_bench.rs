use criterion::{black_box, criterion_group, criterion_main, Criterion};
use giztoy_voiceprint::{compute_fbank, FbankConfig, Hasher, VoiceprintModel};

fn make_sine_pcm(freq_hz: f64, n_samples: usize, sample_rate: usize) -> Vec<u8> {
    let mut audio = vec![0u8; n_samples * 2];
    for i in 0..n_samples {
        let t = i as f64 / sample_rate as f64;
        let sample = (16000.0 * (freq_hz * 2.0 * std::f64::consts::PI * t).sin()) as i16;
        audio[2 * i] = sample as u8;
        audio[2 * i + 1] = (sample >> 8) as u8;
    }
    audio
}

fn bench_fbank(c: &mut Criterion) {
    let cfg = FbankConfig::default();
    let audio = make_sine_pcm(440.0, 6400, 16000); // 400ms

    c.bench_function("voiceprint_fbank_400ms", |b| {
        b.iter(|| {
            let _ = black_box(compute_fbank(black_box(&audio), &cfg));
        });
    });
}

fn bench_fbank_1s(c: &mut Criterion) {
    let cfg = FbankConfig::default();
    let audio = make_sine_pcm(440.0, 16000, 16000); // 1s

    c.bench_function("voiceprint_fbank_1s", |b| {
        b.iter(|| {
            let _ = black_box(compute_fbank(black_box(&audio), &cfg));
        });
    });
}

fn bench_hash(c: &mut Criterion) {
    let h = Hasher::default_512();
    let emb: Vec<f32> = (0..512).map(|i| i as f32 * 0.01).collect();

    c.bench_function("voiceprint_hash_512d_16bit", |b| {
        b.iter(|| {
            let _ = black_box(h.hash(black_box(&emb)));
        });
    });
}

fn bench_ncnn_model_extract(c: &mut Criterion) {
    giztoy_ncnn::register_embedded_models();
    let net = giztoy_ncnn::load_model(giztoy_ncnn::ModelId::SPEAKER_ERES2NET).unwrap();
    let model = giztoy_voiceprint::NCNNModel::from_net(
        net,
        giztoy_voiceprint::NCNNModelConfig::default(),
    );
    let audio = make_sine_pcm(440.0, 6400, 16000); // 400ms

    c.bench_function("voiceprint_ncnn_extract_400ms", |b| {
        b.iter(|| {
            let _ = black_box(model.extract(black_box(&audio)));
        });
    });
}

fn bench_ncnn_model_extract_1s(c: &mut Criterion) {
    giztoy_ncnn::register_embedded_models();
    let net = giztoy_ncnn::load_model(giztoy_ncnn::ModelId::SPEAKER_ERES2NET).unwrap();
    let model = giztoy_voiceprint::NCNNModel::from_net(
        net,
        giztoy_voiceprint::NCNNModelConfig::default(),
    );
    let audio = make_sine_pcm(440.0, 16000, 16000); // 1s

    c.bench_function("voiceprint_ncnn_extract_1s", |b| {
        b.iter(|| {
            let _ = black_box(model.extract(black_box(&audio)));
        });
    });
}

criterion_group!(
    benches,
    bench_fbank,
    bench_fbank_1s,
    bench_hash,
    bench_ncnn_model_extract,
    bench_ncnn_model_extract_1s,
);
criterion_main!(benches);
