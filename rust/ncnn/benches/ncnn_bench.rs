use criterion::{black_box, criterion_group, criterion_main, Criterion};
use giztoy_ncnn::{load_model, register_embedded_models, Mat, ModelId};

fn bench_load_model(c: &mut Criterion) {
    register_embedded_models();
    let mut group = c.benchmark_group("ncnn_load_model");
    for id in [ModelId::SPEAKER_ERES2NET, ModelId::VAD_SILERO, ModelId::DENOISE_NSNET2] {
        group.bench_function(id, |b| {
            b.iter(|| {
                let net = load_model(black_box(id)).unwrap();
                drop(net);
            });
        });
    }
    group.finish();
}

fn bench_speaker_inference(c: &mut Criterion) {
    register_embedded_models();
    let net = load_model(ModelId::SPEAKER_ERES2NET).unwrap();
    let mut data = vec![0.0f32; 40 * 80];
    for (i, v) in data.iter_mut().enumerate() {
        *v = (i % 100) as f32 * 0.01;
    }

    c.bench_function("ncnn_speaker_inference", |b| {
        b.iter(|| {
            let input = Mat::new_2d(80, 40, black_box(&data)).unwrap();
            let mut ex = net.extractor().unwrap();
            ex.set_input("in0", &input).unwrap();
            let output = ex.extract("out0").unwrap();
            let _ = black_box(output.float_data());
        });
    });
}

fn bench_vad_inference(c: &mut Criterion) {
    register_embedded_models();
    let net = load_model(ModelId::VAD_SILERO).unwrap();
    let audio = vec![0.0f32; 512];
    let h = vec![0.0f32; 128];
    let cc = vec![0.0f32; 128];

    c.bench_function("ncnn_vad_inference", |b| {
        b.iter(|| {
            let in_audio = Mat::new_2d(512, 1, black_box(&audio)).unwrap();
            let in_h = Mat::new_2d(128, 1, black_box(&h)).unwrap();
            let in_c = Mat::new_2d(128, 1, black_box(&cc)).unwrap();
            let mut ex = net.extractor().unwrap();
            ex.set_input("in0", &in_audio).unwrap();
            ex.set_input("in1", &in_h).unwrap();
            ex.set_input("in2", &in_c).unwrap();
            let prob = ex.extract("out0").unwrap();
            let _ = black_box(prob.float_data());
        });
    });
}

fn bench_nsnet2_inference(c: &mut Criterion) {
    register_embedded_models();
    let net = load_model(ModelId::DENOISE_NSNET2).unwrap();
    let feat = vec![0.0f32; 161];
    let h1 = vec![0.0f32; 400];
    let h2 = vec![0.0f32; 400];

    c.bench_function("ncnn_nsnet2_inference", |b| {
        b.iter(|| {
            let in_feat = Mat::new_2d(161, 1, black_box(&feat)).unwrap();
            let in_h1 = Mat::new_2d(400, 1, black_box(&h1)).unwrap();
            let in_h2 = Mat::new_2d(400, 1, black_box(&h2)).unwrap();
            let mut ex = net.extractor().unwrap();
            ex.set_input("in0", &in_feat).unwrap();
            ex.set_input("in1", &in_h1).unwrap();
            ex.set_input("in2", &in_h2).unwrap();
            let mask = ex.extract("out0").unwrap();
            let _ = black_box(mask.float_data());
        });
    });
}

fn bench_concurrent_speaker(c: &mut Criterion) {
    register_embedded_models();
    let net = load_model(ModelId::SPEAKER_ERES2NET).unwrap();
    let net = std::sync::Arc::new(net);
    let mut data = vec![0.0f32; 40 * 80];
    for (i, v) in data.iter_mut().enumerate() {
        *v = (i % 100) as f32 * 0.01;
    }

    c.bench_function("ncnn_concurrent_speaker_4threads", |b| {
        b.iter(|| {
            let handles: Vec<_> = (0..4)
                .map(|_| {
                    let net = net.clone();
                    let data = data.clone();
                    std::thread::spawn(move || {
                        let input = Mat::new_2d(80, 40, &data).unwrap();
                        let mut ex = net.extractor().unwrap();
                        ex.set_input("in0", &input).unwrap();
                        let output = ex.extract("out0").unwrap();
                        let _ = black_box(output.float_data());
                    })
                })
                .collect();
            for h in handles {
                h.join().unwrap();
            }
        });
    });
}

criterion_group!(
    benches,
    bench_load_model,
    bench_speaker_inference,
    bench_vad_inference,
    bench_nsnet2_inference,
    bench_concurrent_speaker,
);
criterion_main!(benches);
