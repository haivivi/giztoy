/// Compute the cosine distance between two vectors.
///
/// Returns a value in `[0, 2]` where 0 means identical direction and
/// 2 means opposite direction.
///
/// Uses f64 intermediate precision (matching Go implementation).
/// Returns 2.0 for zero vectors or dimension mismatches.
pub fn cosine_distance(a: &[f32], b: &[f32]) -> f32 {
    if a.len() != b.len() {
        return 2.0;
    }

    let mut dot: f64 = 0.0;
    let mut norm_a: f64 = 0.0;
    let mut norm_b: f64 = 0.0;

    for i in 0..a.len() {
        let ai = a[i] as f64;
        let bi = b[i] as f64;
        dot += ai * bi;
        norm_a += ai * ai;
        norm_b += bi * bi;
    }

    if norm_a == 0.0 || norm_b == 0.0 {
        return 2.0;
    }

    let similarity = dot / (norm_a.sqrt() * norm_b.sqrt());
    // Clamp to [-1, 1] to handle floating point errors.
    let similarity = similarity.clamp(-1.0, 1.0);
    (1.0 - similarity) as f32
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_identical() {
        let d = cosine_distance(&[1.0, 0.0, 0.0], &[1.0, 0.0, 0.0]);
        assert!((d - 0.0).abs() < 0.001, "identical: got {d}");
    }

    #[test]
    fn test_orthogonal() {
        let d = cosine_distance(&[1.0, 0.0, 0.0], &[0.0, 1.0, 0.0]);
        assert!((d - 1.0).abs() < 0.001, "orthogonal: got {d}");
    }

    #[test]
    fn test_opposite() {
        let d = cosine_distance(&[1.0, 0.0, 0.0], &[-1.0, 0.0, 0.0]);
        assert!((d - 2.0).abs() < 0.001, "opposite: got {d}");
    }

    #[test]
    fn test_similar() {
        let d = cosine_distance(&[1.0, 0.1, 0.0], &[1.0, 0.0, 0.0]);
        assert!((d - 0.005).abs() < 0.01, "similar: got {d}");
    }

    #[test]
    fn test_dimension_mismatch() {
        assert_eq!(cosine_distance(&[1.0, 0.0], &[1.0, 0.0, 0.0]), 2.0);
    }

    #[test]
    fn test_zero_vector() {
        assert_eq!(cosine_distance(&[0.0, 0.0, 0.0], &[1.0, 0.0, 0.0]), 2.0);
    }
}
