use criterion::{black_box, criterion_group, criterion_main, Criterion};
use giztoy_onnx::{register_embedded_models, Env, ModelId, Tensor};

fn bench_speaker_inference(c: &mut Criterion) {
    register_embedded_models();
    let env = Env::new("bench").unwrap();
    let session = giztoy_onnx::load_model(&env, ModelId::SPEAKER_ERES2NET).unwrap();

    let mut data = vec![0.0f32; 1 * 40 * 80];
    for (i, v) in data.iter_mut().enumerate() {
        *v = (i % 100) as f32 * 0.01;
    }

    c.bench_function("onnx_speaker_inference", |b| {
        b.iter(|| {
            let input = Tensor::new(&[1, 40, 80], black_box(&data)).unwrap();
            let outputs = session.run(&["x"], &[&input], &["embedding"]).unwrap();
            let _ = black_box(outputs[0].float_data());
        });
    });
}

fn bench_nsnet2_inference(c: &mut Criterion) {
    register_embedded_models();
    let env = Env::new("bench_nsnet2").unwrap();
    let session = giztoy_onnx::load_model(&env, ModelId::DENOISE_NSNET2).unwrap();

    let mut data = vec![0.0f32; 1 * 5 * 161];
    for (i, v) in data.iter_mut().enumerate() {
        *v = (i % 161) as f32 * -0.05;
    }

    c.bench_function("onnx_nsnet2_inference", |b| {
        b.iter(|| {
            let input = Tensor::new(&[1, 5, 161], black_box(&data)).unwrap();
            let outputs = session.run(&["input"], &[&input], &["output"]).unwrap();
            let _ = black_box(outputs[0].float_data());
        });
    });
}

criterion_group!(benches, bench_speaker_inference, bench_nsnet2_inference);
criterion_main!(benches);
