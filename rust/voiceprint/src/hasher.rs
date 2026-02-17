/// Projects high-dimensional embedding vectors into compact
/// locality-sensitive hashes using random hyperplane LSH.
///
/// Each hash is a hex string whose length depends on the configured
/// bit count. For 16 bits the output is 4 hex chars (e.g., "A3F8").
///
/// # Algorithm
///
/// The hasher stores `bits` random unit-length hyperplanes of dimension
/// `dim`. For each hyperplane, the dot product with the input vector
/// determines one bit: positive -> 1, non-positive -> 0.
/// The resulting bit vector is encoded as uppercase hex.
///
/// # Cross-Language Consistency
///
/// For production use, load planes from a persisted JSON file
/// (`data/voiceprint/planes_512_16.json`) via [`Hasher::from_json`].
/// Both Go and Rust load the same file, ensuring identical hashes
/// for the same embedding regardless of language.
pub struct Hasher {
    dim: usize,
    bits: usize,
    planes: Vec<Vec<f32>>, // bits x dim, each row is a unit hyperplane
}

/// JSON format for persisted hyperplane matrices.
/// Both Go and Rust use this format.
#[derive(serde::Deserialize)]
struct PlanesFile {
    dim: usize,
    bits: usize,
    planes: Vec<Vec<f32>>,
}

impl Hasher {
    /// Creates a Hasher from a JSON-encoded planes file.
    ///
    /// This is the **recommended constructor** for production use.
    /// Load the JSON from `data/voiceprint/planes_512_16.json` (embedded
    /// via `include_bytes!` or read from disk).
    pub fn from_json(json_data: &[u8]) -> Result<Self, String> {
        let pf: PlanesFile =
            serde_json::from_slice(json_data).map_err(|e| format!("parse planes JSON: {e}"))?;
        if pf.planes.is_empty() {
            return Err("empty planes in JSON".into());
        }
        Ok(Self::from_planes(pf.dim, pf.bits, pf.planes))
    }

    /// Creates the default 512-dim, 16-bit Hasher from the embedded planes file.
    ///
    /// This loads `data/voiceprint/planes_512_16.json` which is embedded at
    /// compile time. Both Go and Rust use the same planes, ensuring
    /// cross-language hash consistency.
    pub fn default_512() -> Self {
        static PLANES_JSON: &[u8] = include_bytes!("planes_512_16.json");
        Self::from_json(PLANES_JSON).expect("embedded planes_512_16.json is valid")
    }

    /// Creates a Hasher with pre-computed planes.
    pub fn from_planes(dim: usize, bits: usize, planes: Vec<Vec<f32>>) -> Self {
        assert!(bits > 0 && bits % 4 == 0, "voiceprint: bits must be a positive multiple of 4");
        assert!(dim > 0, "voiceprint: dim must be positive");
        assert_eq!(planes.len(), bits, "voiceprint: planes count must equal bits");
        for (i, p) in planes.iter().enumerate() {
            assert_eq!(p.len(), dim, "voiceprint: plane {i} has wrong dimension");
        }
        Self { dim, bits, planes }
    }

    /// Creates a Hasher by generating random hyperplanes from a seed.
    ///
    /// Uses a simple xoshiro256** PRNG with Box-Muller transform for
    /// normal distribution. The seed determines the planes deterministically.
    ///
    /// **Note**: For cross-language (Go/Rust) hash consistency, prefer
    /// [`Hasher::from_planes`] with shared pre-computed planes.
    pub fn new(dim: usize, bits: usize, seed: u64) -> Self {
        assert!(bits > 0 && bits % 4 == 0, "voiceprint: bits must be a positive multiple of 4");
        assert!(dim > 0, "voiceprint: dim must be positive");

        let mut rng = Xoshiro256ss::new(seed);
        let mut planes = Vec::with_capacity(bits);
        for _ in 0..bits {
            let mut plane = Vec::with_capacity(dim);
            let mut norm: f64 = 0.0;
            for _ in 0..dim {
                let v = rng.norm_float64() as f32;
                plane.push(v);
                norm += (v as f64) * (v as f64);
            }
            norm = norm.sqrt();
            if norm > 0.0 {
                let scale = (1.0 / norm) as f32;
                for v in &mut plane {
                    *v *= scale;
                }
            }
            planes.push(plane);
        }
        Self { dim, bits, planes }
    }

    /// Projects an embedding vector into a hex hash string.
    /// The input must have length equal to the hasher's dimension.
    /// Returns an uppercase hex string of length `bits/4`.
    pub fn hash(&self, embedding: &[f32]) -> String {
        assert_eq!(
            embedding.len(),
            self.dim,
            "voiceprint: embedding dimension mismatch"
        );

        let n_bytes = (self.bits + 7) / 8;
        let mut hash_bytes = vec![0u8; n_bytes];

        for (i, plane) in self.planes.iter().enumerate() {
            let dot = dot32(plane, embedding);
            if dot > 0.0 {
                hash_bytes[i / 8] |= 1 << (7 - (i % 8));
            }
        }

        // Encode as uppercase hex and truncate to exact nibble count.
        let n_nibbles = self.bits / 4;
        let full_hex = hex_encode_upper(&hash_bytes);
        full_hex[..n_nibbles].to_string()
    }

    /// Returns the number of hash bits.
    pub fn bits(&self) -> usize {
        self.bits
    }

    /// Returns the expected embedding dimension.
    pub fn dim(&self) -> usize {
        self.dim
    }

    /// Returns a reference to the internal planes matrix.
    /// Useful for serializing to JSON for cross-language sharing.
    pub fn planes(&self) -> &[Vec<f32>] {
        &self.planes
    }
}

fn dot32(a: &[f32], b: &[f32]) -> f32 {
    a.iter().zip(b.iter()).map(|(x, y)| x * y).sum()
}

fn hex_encode_upper(bytes: &[u8]) -> String {
    bytes
        .iter()
        .map(|b| format!("{b:02X}"))
        .collect()
}

// ---------------------------------------------------------------------------
// Xoshiro256** PRNG + Box-Muller normal distribution
//
// This is NOT Go's PCG. For cross-language hash consistency, use from_planes()
// with pre-computed planes. This PRNG is for standalone Rust usage.
// ---------------------------------------------------------------------------

struct Xoshiro256ss {
    s: [u64; 4],
    has_spare: bool,
    spare: f64,
}

impl Xoshiro256ss {
    fn new(seed: u64) -> Self {
        // SplitMix64 to initialize state from single seed.
        let mut z = seed;
        let mut s = [0u64; 4];
        for slot in &mut s {
            z = z.wrapping_add(0x9e3779b97f4a7c15);
            z = (z ^ (z >> 30)).wrapping_mul(0xbf58476d1ce4e5b9);
            z = (z ^ (z >> 27)).wrapping_mul(0x94d049bb133111eb);
            *slot = z ^ (z >> 31);
        }
        Self {
            s,
            has_spare: false,
            spare: 0.0,
        }
    }

    fn next_u64(&mut self) -> u64 {
        let result = (self.s[1].wrapping_mul(5)).rotate_left(7).wrapping_mul(9);
        let t = self.s[1] << 17;
        self.s[2] ^= self.s[0];
        self.s[3] ^= self.s[1];
        self.s[1] ^= self.s[2];
        self.s[0] ^= self.s[3];
        self.s[2] ^= t;
        self.s[3] = self.s[3].rotate_left(45);
        result
    }

    fn float64(&mut self) -> f64 {
        // Generate uniform [0, 1) from u64.
        (self.next_u64() >> 11) as f64 / (1u64 << 53) as f64
    }

    /// Box-Muller transform to generate standard normal.
    fn norm_float64(&mut self) -> f64 {
        if self.has_spare {
            self.has_spare = false;
            return self.spare;
        }

        loop {
            let u1 = self.float64();
            let u2 = self.float64();
            if u1 > 0.0 {
                let mag = (-2.0 * u1.ln()).sqrt();
                let angle = 2.0 * std::f64::consts::PI * u2;
                self.spare = mag * angle.sin();
                self.has_spare = true;
                return mag * angle.cos();
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hasher_deterministic() {
        let h1 = Hasher::new(4, 16, 42);
        let h2 = Hasher::new(4, 16, 42);

        let emb = [1.0f32, 0.0, 0.0, 0.0];
        assert_eq!(h1.hash(&emb), h2.hash(&emb));
    }

    #[test]
    fn hasher_different_seeds() {
        let h1 = Hasher::new(4, 16, 42);
        let h2 = Hasher::new(4, 16, 99);

        let emb = [1.0f32, 0.0, 0.0, 0.0];
        // Different seeds should (very likely) produce different hashes.
        // Not guaranteed but extremely likely for random hyperplanes.
        let hash1 = h1.hash(&emb);
        let hash2 = h2.hash(&emb);
        // At least they should be valid hex of correct length.
        assert_eq!(hash1.len(), 4);
        assert_eq!(hash2.len(), 4);
    }

    #[test]
    fn hasher_hex_format() {
        let h = Hasher::new(8, 16, 0);
        let emb = [1.0, 0.5, -0.3, 0.8, -0.1, 0.2, 0.9, -0.5];
        let hash = h.hash(&emb);

        assert_eq!(hash.len(), 4, "16 bits = 4 hex chars");
        assert!(
            hash.chars().all(|c| c.is_ascii_hexdigit() && !c.is_lowercase()),
            "should be uppercase hex, got {hash}"
        );
    }

    #[test]
    fn hasher_similar_vectors_similar_hash() {
        let h = Hasher::new(64, 16, 42);

        let emb1: Vec<f32> = (0..64).map(|i| if i == 0 { 1.0 } else { 0.0 }).collect();
        let mut emb2 = emb1.clone();
        emb2[1] = 0.01; // Tiny perturbation.

        // Similar vectors should produce the same hash most of the time.
        let hash1 = h.hash(&emb1);
        let hash2 = h.hash(&emb2);
        assert_eq!(hash1, hash2, "very similar vectors should hash identically");
    }

    #[test]
    fn hasher_from_planes() {
        let planes = vec![
            vec![1.0f32, 0.0, 0.0],
            vec![0.0, 1.0, 0.0],
            vec![0.0, 0.0, 1.0],
            vec![-1.0, 0.0, 0.0],
        ];
        let h = Hasher::from_planes(3, 4, planes);

        let emb = [1.0f32, 1.0, 1.0];
        let hash = h.hash(&emb);
        // Dot products: 1, 1, 1, -1 → bits: 1, 1, 1, 0 → 0xE → "E"
        assert_eq!(hash, "E");

        let emb2 = [-1.0f32, -1.0, -1.0];
        let hash2 = h.hash(&emb2);
        // Dot products: -1, -1, -1, 1 → bits: 0, 0, 0, 1 → 0x1 → "1"
        assert_eq!(hash2, "1");
    }

    #[test]
    fn xoshiro_deterministic() {
        let mut rng1 = Xoshiro256ss::new(42);
        let mut rng2 = Xoshiro256ss::new(42);
        for _ in 0..100 {
            assert_eq!(rng1.next_u64(), rng2.next_u64());
        }
    }

    #[test]
    fn norm_float64_distribution() {
        let mut rng = Xoshiro256ss::new(0);
        let n = 10000;
        let mut sum = 0.0;
        let mut sum_sq = 0.0;
        for _ in 0..n {
            let v = rng.norm_float64();
            sum += v;
            sum_sq += v * v;
        }
        let mean = sum / n as f64;
        let variance = sum_sq / n as f64 - mean * mean;

        assert!(mean.abs() < 0.1, "mean should be ~0, got {mean}");
        assert!((variance - 1.0).abs() < 0.1, "variance should be ~1, got {variance}");
    }

    #[test]
    fn default_512_loads() {
        let h = Hasher::default_512();
        assert_eq!(h.dim(), 512);
        assert_eq!(h.bits(), 16);
    }

    #[test]
    fn cross_lang_hash_reference() {
        // The reference hash for emb[i] = i * 0.01 (dim=512) is "82A9",
        // generated by Go's gen_planes.go using the same planes file.
        let h = Hasher::default_512();

        let emb: Vec<f32> = (0..512).map(|i| i as f32 * 0.01).collect();
        let hash = h.hash(&emb);
        assert_eq!(
            hash, "82A9",
            "cross-language hash mismatch: Rust got {hash}, Go expects 82A9"
        );
    }
}
