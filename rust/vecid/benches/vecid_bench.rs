use criterion::{black_box, criterion_group, criterion_main, Criterion};
use giztoy_vecid::{Config, MemoryStore, Registry};

fn random_unit_vec(dim: usize, seed: u64) -> Vec<f32> {
    let mut v = Vec::with_capacity(dim);
    let mut state = seed;
    for _ in 0..dim {
        state = state.wrapping_mul(6364136223846793005).wrapping_add(1);
        v.push(((state >> 33) as f32) / (u32::MAX as f32) - 0.5);
    }
    let norm: f64 = v.iter().map(|&x| (x as f64) * (x as f64)).sum::<f64>().sqrt();
    if norm > 0.0 {
        let s = (1.0 / norm) as f32;
        for x in &mut v {
            *x *= s;
        }
    }
    v
}

fn make_cluster(centroid: &[f32], n: usize, noise: f64, base_seed: u64) -> Vec<Vec<f32>> {
    let dim = centroid.len();
    (0..n)
        .map(|i| {
            let mut v = centroid.to_vec();
            let rvec = random_unit_vec(dim, base_seed.wrapping_add(i as u64 * 997));
            for (j, x) in v.iter_mut().enumerate() {
                *x += rvec[j] * noise as f32;
            }
            let norm: f64 = v.iter().map(|&x| (x as f64) * (x as f64)).sum::<f64>().sqrt();
            if norm > 0.0 {
                let s = (1.0 / norm) as f32;
                for x in &mut v {
                    *x *= s;
                }
            }
            v
        })
        .collect()
}

fn bench_identify(c: &mut Criterion) {
    let dim = 512;
    let reg = Registry::with_memory_store(Config {
        dim,
        threshold: 0.5,
        min_samples: 2,
        prefix: "s".into(),
    });

    let c1 = random_unit_vec(dim, 1);
    let c2 = random_unit_vec(dim, 2);
    for emb in make_cluster(&c1, 20, 0.1, 100) {
        reg.identify(&emb);
    }
    for emb in make_cluster(&c2, 20, 0.1, 200) {
        reg.identify(&emb);
    }
    reg.recluster();

    let test_emb = random_unit_vec(dim, 999);

    c.bench_function("vecid_identify_512d_2buckets", |b| {
        b.iter(|| {
            let _ = black_box(reg.identify(black_box(&test_emb)));
        });
    });
}

fn bench_recluster(c: &mut Criterion) {
    let dim = 512;

    c.bench_function("vecid_recluster_512d_60points_3clusters", |b| {
        b.iter_with_setup(
            || {
                let reg = Registry::new(
                    Config {
                        dim,
                        threshold: 0.5,
                        min_samples: 2,
                        prefix: "s".into(),
                    },
                    Box::new(MemoryStore::new()),
                );
                let c1 = random_unit_vec(dim, 10);
                let c2 = random_unit_vec(dim, 20);
                let c3 = random_unit_vec(dim, 30);
                for emb in make_cluster(&c1, 20, 0.1, 100) {
                    reg.identify(&emb);
                }
                for emb in make_cluster(&c2, 20, 0.1, 200) {
                    reg.identify(&emb);
                }
                for emb in make_cluster(&c3, 20, 0.1, 300) {
                    reg.identify(&emb);
                }
                reg
            },
            |reg| {
                let _ = black_box(reg.recluster());
            },
        );
    });
}

criterion_group!(benches, bench_identify, bench_recluster);
criterion_main!(benches);
