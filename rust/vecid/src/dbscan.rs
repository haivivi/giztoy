/// Runs the DBSCAN clustering algorithm using cosine distance.
///
/// # Parameters
/// - `vectors`: the data points (L2-normalized embeddings)
/// - `eps`: maximum cosine distance (1 - cosine_similarity) for neighbors
/// - `min_pts`: minimum points to form a dense cluster
///
/// # Returns
/// Cluster labels for each vector. Label -1 means noise (unassigned).
/// Positive labels (1, 2, ...) identify clusters.
pub(crate) fn dbscan(vectors: &[&[f32]], eps: f32, min_pts: usize) -> Vec<i32> {
    let n = vectors.len();
    if n == 0 {
        return Vec::new();
    }

    const UNDEFINED: i32 = 0;
    const NOISE: i32 = -1;

    let mut labels = vec![UNDEFINED; n];
    let mut cluster_id: i32 = 0;

    for i in 0..n {
        if labels[i] != UNDEFINED {
            continue;
        }

        let neighbors = range_query(vectors, i, eps);
        if neighbors.len() < min_pts {
            labels[i] = NOISE;
            continue;
        }

        // Start a new cluster.
        cluster_id += 1;
        labels[i] = cluster_id;

        // Seed set: neighbors minus point i.
        let mut seed: Vec<usize> = neighbors.into_iter().filter(|&j| j != i).collect();

        while let Some(q) = seed.first().copied() {
            seed.remove(0);

            if labels[q] == NOISE {
                labels[q] = cluster_id;
            }
            if labels[q] != UNDEFINED {
                continue;
            }
            labels[q] = cluster_id;

            let q_neighbors = range_query(vectors, q, eps);
            if q_neighbors.len() >= min_pts {
                seed.extend(q_neighbors);
            }
        }
    }

    labels
}

/// Returns indices of all vectors within eps cosine distance of vectors[idx].
fn range_query(vectors: &[&[f32]], idx: usize, eps: f32) -> Vec<usize> {
    let q = vectors[idx];
    vectors
        .iter()
        .enumerate()
        .filter(|(_, v)| cosine_distance(q, v) <= eps)
        .map(|(i, _)| i)
        .collect()
}

/// Cosine distance: 1 - cosine_similarity.
fn cosine_distance(a: &[f32], b: &[f32]) -> f32 {
    1.0 - cosine_sim(a, b)
}

/// Cosine similarity between two vectors.
/// Uses f64 intermediate precision to match Go implementation.
pub(crate) fn cosine_sim(a: &[f32], b: &[f32]) -> f32 {
    let mut dot: f64 = 0.0;
    let mut na: f64 = 0.0;
    let mut nb: f64 = 0.0;
    for i in 0..a.len() {
        let ai = a[i] as f64;
        let bi = b[i] as f64;
        dot += ai * bi;
        na += ai * ai;
        nb += bi * bi;
    }
    let denom = na.sqrt() * nb.sqrt();
    if denom == 0.0 {
        return 0.0;
    }
    (dot / denom) as f32
}

/// Normalizes a vector to unit length in-place.
pub(crate) fn l2_norm(v: &mut [f32]) {
    let mut sum: f64 = 0.0;
    for &x in v.iter() {
        sum += (x as f64) * (x as f64);
    }
    let norm = sum.sqrt();
    if norm > 0.0 {
        let scale = (1.0 / norm) as f32;
        for x in v.iter_mut() {
            *x *= scale;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cosine_sim_identical() {
        let a = [1.0, 0.0, 0.0];
        let b = [1.0, 0.0, 0.0];
        let sim = cosine_sim(&a, &b);
        assert!((sim - 1.0).abs() < 1e-6, "identical vectors should have sim=1, got {sim}");
    }

    #[test]
    fn cosine_sim_orthogonal() {
        let a = [1.0, 0.0, 0.0];
        let b = [0.0, 1.0, 0.0];
        let sim = cosine_sim(&a, &b);
        assert!(sim.abs() < 1e-6, "orthogonal vectors should have sim=0, got {sim}");
    }

    #[test]
    fn cosine_sim_opposite() {
        let a = [1.0, 0.0, 0.0];
        let b = [-1.0, 0.0, 0.0];
        let sim = cosine_sim(&a, &b);
        assert!((sim + 1.0).abs() < 1e-6, "opposite vectors should have sim=-1, got {sim}");
    }

    #[test]
    fn l2_norm_unit() {
        let mut v = [3.0, 4.0];
        l2_norm(&mut v);
        let norm: f64 = v.iter().map(|&x| (x as f64) * (x as f64)).sum::<f64>().sqrt();
        assert!((norm - 1.0).abs() < 1e-6, "should be unit length, got {norm}");
        assert!((v[0] - 0.6).abs() < 1e-6);
        assert!((v[1] - 0.8).abs() < 1e-6);
    }

    #[test]
    fn l2_norm_zero() {
        let mut v = [0.0, 0.0, 0.0];
        l2_norm(&mut v);
        assert_eq!(v, [0.0, 0.0, 0.0]);
    }

    #[test]
    fn dbscan_basic_two_clusters() {
        // Two tight clusters far apart.
        let cluster_a: Vec<Vec<f32>> = vec![
            vec![1.0, 0.0, 0.0],
            vec![0.99, 0.1, 0.0],
            vec![0.98, 0.15, 0.0],
        ];
        let cluster_b: Vec<Vec<f32>> = vec![
            vec![0.0, 1.0, 0.0],
            vec![0.1, 0.99, 0.0],
            vec![0.15, 0.98, 0.0],
        ];

        let mut all: Vec<Vec<f32>> = Vec::new();
        all.extend(cluster_a.iter().map(|v| {
            let mut c = v.clone();
            l2_norm(&mut c);
            c
        }));
        all.extend(cluster_b.iter().map(|v| {
            let mut c = v.clone();
            l2_norm(&mut c);
            c
        }));

        let refs: Vec<&[f32]> = all.iter().map(|v| v.as_slice()).collect();
        let labels = dbscan(&refs, 0.3, 2);

        // First 3 should be one cluster, last 3 another.
        assert_eq!(labels.len(), 6);
        assert!(labels[0] > 0);
        assert_eq!(labels[0], labels[1]);
        assert_eq!(labels[0], labels[2]);
        assert!(labels[3] > 0);
        assert_eq!(labels[3], labels[4]);
        assert_eq!(labels[3], labels[5]);
        assert_ne!(labels[0], labels[3]);
    }

    #[test]
    fn dbscan_noise() {
        // Single point cannot form a cluster with min_pts=2.
        let vectors: Vec<Vec<f32>> = vec![vec![1.0, 0.0, 0.0]];
        let refs: Vec<&[f32]> = vectors.iter().map(|v| v.as_slice()).collect();
        let labels = dbscan(&refs, 0.1, 2);
        assert_eq!(labels, vec![-1]);
    }

    #[test]
    fn dbscan_empty() {
        let labels = dbscan(&[], 0.1, 2);
        assert!(labels.is_empty());
    }
}
