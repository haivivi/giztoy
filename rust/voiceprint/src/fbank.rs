use std::f64::consts::PI;

/// Configures mel filterbank feature extraction.
///
/// Default configuration matches Kaldi/sherpa-onnx for speaker embedding models:
/// Povey window, 25ms frames, 10ms shift, 80 mel bins, 20-7600 Hz range.
#[derive(Debug, Clone)]
pub struct FbankConfig {
    /// Input sample rate in Hz (default: 16000).
    pub sample_rate: usize,
    /// Number of mel filterbank channels (default: 80).
    pub num_mels: usize,
    /// Frame length in samples (default: 400 = 25ms @ 16kHz).
    pub frame_length: usize,
    /// Frame shift in samples (default: 160 = 10ms @ 16kHz).
    pub frame_shift: usize,
    /// Pre-emphasis coefficient (default: 0.97).
    pub pre_emphasis: f64,
    /// Floor for log energy (default: 1e-10).
    pub energy_floor: f64,
    /// Low cutoff frequency for mel bins (default: 20 Hz).
    pub low_freq: f64,
    /// High cutoff frequency, negative = offset from Nyquist (default: -400).
    pub high_freq: f64,
    /// Remove DC offset per frame (default: true).
    pub remove_dc: bool,
    /// Use Povey window (hamming^0.85) instead of Hamming (default: true).
    pub povey_window: bool,
    /// Normalize PCM16 samples to [-1, 1] range (default: true).
    pub normalize_pcm: bool,
}

impl Default for FbankConfig {
    fn default() -> Self {
        Self {
            sample_rate: 16000,
            num_mels: 80,
            frame_length: 400,  // 25ms @ 16kHz
            frame_shift: 160,   // 10ms @ 16kHz
            pre_emphasis: 0.97,
            energy_floor: 1e-10,
            low_freq: 20.0,     // Kaldi default
            high_freq: -400.0,  // Nyquist - 400 = 7600 Hz for 16kHz
            remove_dc: true,
            povey_window: true,
            normalize_pcm: true,
        }
    }
}

/// Extracts log mel filterbank features from PCM16 audio.
///
/// Input: PCM16 signed little-endian audio bytes at the configured sample rate.
/// Output: 2D vec `[num_frames][num_mels]` of log mel filterbank energies.
///
/// Returns `None` if the audio is too short for a single frame.
pub fn compute_fbank(audio: &[u8], cfg: &FbankConfig) -> Option<Vec<Vec<f32>>> {
    if cfg.frame_shift == 0 || cfg.frame_length == 0 || cfg.num_mels == 0 {
        return None;
    }

    // Convert PCM16 to f64 samples.
    let n_samples = audio.len() / 2;
    if n_samples < cfg.frame_length {
        return None;
    }
    let mut samples = Vec::with_capacity(n_samples);
    for i in 0..n_samples {
        let lo = audio[2 * i] as u8;
        let hi = audio[2 * i + 1] as u8;
        let s = (lo as i16) | ((hi as i16) << 8);
        samples.push(s as f64);
    }

    // Normalize to [-1, 1].
    if cfg.normalize_pcm {
        for s in &mut samples {
            *s /= 32768.0;
        }
    }

    let num_frames = (n_samples - cfg.frame_length) / cfg.frame_shift + 1;
    if num_frames == 0 {
        return None;
    }

    // FFT size: next power of 2 >= frame_length.
    let fft_size = next_pow2(cfg.frame_length);
    let half_fft = fft_size / 2 + 1;

    // Pre-compute window.
    let window = if cfg.povey_window {
        povey_window(cfg.frame_length)
    } else {
        hamming_window(cfg.frame_length)
    };

    // Resolve high frequency.
    let high_freq = if cfg.high_freq <= 0.0 {
        cfg.sample_rate as f64 / 2.0 + cfg.high_freq
    } else {
        cfg.high_freq
    };

    // Pre-compute mel filterbank.
    let filterbank = mel_filterbank(cfg.num_mels, fft_size, cfg.sample_rate, cfg.low_freq, high_freq);

    let mut result = Vec::with_capacity(num_frames);
    let mut fft_buf = vec![(0.0f64, 0.0f64); fft_size];

    for f in 0..num_frames {
        let offset = f * cfg.frame_shift;

        // Extract frame.
        let mut frame_buf: Vec<f64> = samples[offset..offset + cfg.frame_length].to_vec();

        // Remove DC offset.
        if cfg.remove_dc {
            let mean: f64 = frame_buf.iter().sum::<f64>() / cfg.frame_length as f64;
            for v in &mut frame_buf {
                *v -= mean;
            }
        }

        // Pre-emphasis (applied per frame after DC removal).
        if cfg.pre_emphasis > 0.0 {
            for i in (1..cfg.frame_length).rev() {
                frame_buf[i] -= cfg.pre_emphasis * frame_buf[i - 1];
            }
            frame_buf[0] *= 1.0 - cfg.pre_emphasis;
        }

        // Apply window and zero-pad to FFT size.
        for v in &mut fft_buf {
            *v = (0.0, 0.0);
        }
        for i in 0..cfg.frame_length {
            fft_buf[i] = (frame_buf[i] * window[i], 0.0);
        }

        // In-place FFT.
        fft(&mut fft_buf);

        // Power spectrum: |X[k]|^2.
        let mut power_spec = vec![0.0f64; half_fft];
        for k in 0..half_fft {
            let (r, im) = fft_buf[k];
            power_spec[k] = r * r + im * im;
        }

        // Apply mel filterbank and take log.
        let mut frame = vec![0.0f32; cfg.num_mels];
        for m in 0..cfg.num_mels {
            let mut energy: f64 = 0.0;
            for (k, &w) in filterbank[m].iter().enumerate() {
                energy += w * power_spec[k];
            }
            if energy < cfg.energy_floor {
                energy = cfg.energy_floor;
            }
            frame[m] = energy.ln() as f32;
        }
        result.push(frame);
    }

    Some(result)
}

/// CMVN: subtract mean and divide by std per mel bin.
/// Removes channel and environment effects.
pub fn cmvn(features: &mut [Vec<f32>]) {
    if features.is_empty() {
        return;
    }
    let num_mels = features[0].len();
    let t = features.len() as f64;

    for m in 0..num_mels {
        let mut sum: f64 = 0.0;
        for f in features.iter() {
            sum += f[m] as f64;
        }
        let mean = sum / t;

        let mut var_sum: f64 = 0.0;
        for f in features.iter() {
            let d = f[m] as f64 - mean;
            var_sum += d * d;
        }
        let mut std = (var_sum / t).sqrt();
        if std < 1e-10 {
            std = 1e-10;
        }

        for f in features.iter_mut() {
            f[m] = ((f[m] as f64 - mean) / std) as f32;
        }
    }
}

/// L2-normalizes a vector to unit length in-place.
/// Uses f64 intermediate precision to match Go implementation.
pub fn l2_normalize(v: &mut [f32]) {
    let mut norm: f64 = 0.0;
    for &x in v.iter() {
        norm += (x as f64) * (x as f64);
    }
    norm = norm.sqrt();
    if norm > 0.0 {
        let scale = (1.0 / norm) as f32;
        for x in v.iter_mut() {
            *x *= scale;
        }
    }
}

fn next_pow2(n: usize) -> usize {
    let mut p = 1;
    while p < n {
        p <<= 1;
    }
    p
}

fn hamming_window(n: usize) -> Vec<f64> {
    (0..n)
        .map(|i| 0.54 - 0.46 * (2.0 * PI * i as f64 / (n - 1) as f64).cos())
        .collect()
}

/// Povey window (hamming^0.85) used by Kaldi.
fn povey_window(n: usize) -> Vec<f64> {
    hamming_window(n)
        .into_iter()
        .map(|w| w.powf(0.85))
        .collect()
}

fn hz_to_mel(hz: f64) -> f64 {
    2595.0 * (1.0 + hz / 700.0).log10()
}

fn mel_to_hz(mel: f64) -> f64 {
    700.0 * (10.0_f64.powf(mel / 2595.0) - 1.0)
}

/// Computes triangular mel filterbank weights.
/// Returns `[num_mels][half_fft]` weights.
fn mel_filterbank(num_mels: usize, fft_size: usize, sample_rate: usize, low_freq: f64, high_freq: f64) -> Vec<Vec<f64>> {
    let half_fft = fft_size / 2 + 1;
    let mel_low = hz_to_mel(low_freq);
    let mel_high = hz_to_mel(high_freq);

    // Equally spaced mel points.
    let mel_points: Vec<f64> = (0..num_mels + 2)
        .map(|i| mel_low + i as f64 * (mel_high - mel_low) / (num_mels + 1) as f64)
        .collect();

    // Convert back to Hz and then to FFT bin indices.
    let bin_indices: Vec<usize> = mel_points
        .iter()
        .map(|&m| {
            let hz = mel_to_hz(m);
            let bin = (hz * fft_size as f64 / sample_rate as f64).floor() as isize;
            bin.max(0).min(half_fft as isize - 1) as usize
        })
        .collect();

    // Build triangular filters.
    let mut fb = Vec::with_capacity(num_mels);
    for m in 0..num_mels {
        let mut filter = vec![0.0f64; half_fft];
        let left = bin_indices[m];
        let center = bin_indices[m + 1];
        let right = bin_indices[m + 2];

        // Rising slope.
        if center > left {
            for k in left..=center {
                filter[k] = (k - left) as f64 / (center - left) as f64;
            }
        }
        // Falling slope.
        if right > center {
            for k in center..=right {
                filter[k] = (right - k) as f64 / (right - center) as f64;
            }
        }
        fb.push(filter);
    }
    fb
}

/// In-place Cooley-Tukey FFT.
/// Input length must be a power of 2.
/// Uses (real, imag) tuples instead of a complex number type.
fn fft(x: &mut [(f64, f64)]) {
    let n = x.len();
    if n <= 1 {
        return;
    }

    // Bit-reversal permutation.
    let mut j = 0usize;
    for i in 1..n {
        let mut bit = n >> 1;
        while j & bit != 0 {
            j ^= bit;
            bit >>= 1;
        }
        j ^= bit;
        if i < j {
            x.swap(i, j);
        }
    }

    // Butterfly operations.
    let mut size = 2;
    while size <= n {
        let half = size / 2;
        let angle = -2.0 * PI / size as f64;
        let wn = (angle.cos(), angle.sin());
        let mut start = 0;
        while start < n {
            let mut w = (1.0, 0.0);
            for k in 0..half {
                let u = x[start + k];
                // Complex multiply: w * x[start + k + half]
                let t_re = w.0 * x[start + k + half].0 - w.1 * x[start + k + half].1;
                let t_im = w.0 * x[start + k + half].1 + w.1 * x[start + k + half].0;
                x[start + k] = (u.0 + t_re, u.1 + t_im);
                x[start + k + half] = (u.0 - t_re, u.1 - t_im);
                // Complex multiply: w *= wn
                let new_w_re = w.0 * wn.0 - w.1 * wn.1;
                let new_w_im = w.0 * wn.1 + w.1 * wn.0;
                w = (new_w_re, new_w_im);
            }
            start += size;
        }
        size <<= 1;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fbank_config_default() {
        let cfg = FbankConfig::default();
        assert_eq!(cfg.sample_rate, 16000);
        assert_eq!(cfg.num_mels, 80);
        assert_eq!(cfg.frame_length, 400);
        assert_eq!(cfg.frame_shift, 160);
    }

    #[test]
    fn compute_fbank_too_short() {
        let cfg = FbankConfig::default();
        // 100 samples = 200 bytes, need 400 samples = 800 bytes.
        let audio = vec![0u8; 200];
        assert!(compute_fbank(&audio, &cfg).is_none());
    }

    #[test]
    fn compute_fbank_silence() {
        let cfg = FbankConfig::default();
        // 800 samples = 1600 bytes.
        // num_frames = (800 - 400) / 160 + 1 = 3.
        let audio = vec![0u8; 1600];
        let result = compute_fbank(&audio, &cfg);
        assert!(result.is_some());
        let features = result.unwrap();
        assert_eq!(features.len(), 3);
        assert_eq!(features[0].len(), 80);
    }

    #[test]
    fn compute_fbank_multiple_frames() {
        let cfg = FbankConfig::default();
        // 16000 samples = 1 second @ 16kHz = 32000 bytes.
        // Frames: (16000 - 400) / 160 + 1 = 98.
        let mut audio = vec![0u8; 32000];
        // Generate a simple tone.
        for i in 0..16000 {
            let t = i as f64 / 16000.0;
            let sample = (440.0 * 2.0 * PI * t).sin() * 16000.0;
            let s = sample as i16;
            audio[2 * i] = s as u8;
            audio[2 * i + 1] = (s >> 8) as u8;
        }
        let features = compute_fbank(&audio, &cfg).unwrap();
        assert_eq!(features.len(), 98);
        assert_eq!(features[0].len(), 80);

        // Non-silent audio should produce non-uniform features.
        let first_frame = &features[0];
        let not_all_same = first_frame.windows(2).any(|w| (w[0] - w[1]).abs() > 0.01);
        assert!(not_all_same, "tone should produce varied mel energies");
    }

    #[test]
    fn cmvn_normalizes() {
        let mut features = vec![
            vec![1.0f32, 2.0, 3.0],
            vec![3.0, 4.0, 5.0],
            vec![5.0, 6.0, 7.0],
        ];
        cmvn(&mut features);

        // After CMVN, each mel bin should have mean ~0 and std ~1.
        for m in 0..3 {
            let vals: Vec<f64> = features.iter().map(|f| f[m] as f64).collect();
            let mean: f64 = vals.iter().sum::<f64>() / vals.len() as f64;
            assert!(mean.abs() < 1e-5, "mean should be ~0, got {mean}");
        }
    }

    #[test]
    fn l2_normalize_unit() {
        let mut v = vec![3.0f32, 4.0];
        l2_normalize(&mut v);
        let norm: f64 = v.iter().map(|&x| (x as f64) * (x as f64)).sum::<f64>().sqrt();
        assert!((norm - 1.0).abs() < 1e-6);
    }

    #[test]
    fn l2_normalize_zero() {
        let mut v = vec![0.0f32, 0.0, 0.0];
        l2_normalize(&mut v);
        assert_eq!(v, vec![0.0, 0.0, 0.0]);
    }

    #[test]
    fn fft_simple() {
        // FFT of [1,0,0,0] should be [1,1,1,1].
        let mut buf = vec![(1.0, 0.0), (0.0, 0.0), (0.0, 0.0), (0.0, 0.0)];
        fft(&mut buf);
        for (re, im) in &buf {
            assert!((re - 1.0).abs() < 1e-10, "real should be 1, got {re}");
            assert!(im.abs() < 1e-10, "imag should be 0, got {im}");
        }
    }

    #[test]
    fn fft_roundtrip_parseval() {
        // Generate a simple signal and verify Parseval's theorem:
        // sum |x[n]|^2 = (1/N) * sum |X[k]|^2
        let n = 8;
        let mut buf: Vec<(f64, f64)> = (0..n)
            .map(|i| ((2.0 * PI * i as f64 / n as f64).sin(), 0.0))
            .collect();

        let time_energy: f64 = buf.iter().map(|(r, im)| r * r + im * im).sum();
        fft(&mut buf);
        let freq_energy: f64 = buf.iter().map(|(r, im)| r * r + im * im).sum();

        // Parseval: time_energy * N = freq_energy
        assert!(
            (time_energy * n as f64 - freq_energy).abs() < 1e-8,
            "Parseval violated: {} vs {}",
            time_energy * n as f64,
            freq_energy
        );
    }

    #[test]
    fn mel_hz_roundtrip() {
        for &hz in &[0.0, 100.0, 440.0, 1000.0, 8000.0] {
            let mel = hz_to_mel(hz);
            let back = mel_to_hz(mel);
            assert!((hz - back).abs() < 1e-6, "roundtrip failed for {hz}: got {back}");
        }
    }

    /// Cross-language validation: same PCM → same fbank features as Go.
    /// Loads reference.json generated by Go and compares element-by-element.
    #[test]
    fn cross_lang_fbank() {
        let ref_path = std::env::var("TEST_SRCDIR")
            .map(|d| {
                let ws = std::env::var("TEST_WORKSPACE").unwrap_or("_main".into());
                format!("{d}/{ws}/testdata/compat/fbank/reference.json")
            })
            .unwrap_or_else(|_| "testdata/compat/fbank/reference.json".into());

        let json_data = match std::fs::read_to_string(&ref_path) {
            Ok(d) => d,
            Err(_) => {
                eprintln!("fbank reference.json not found at {ref_path}, skipping");
                return;
            }
        };

        #[derive(serde::Deserialize)]
        struct FbankRef {
            num_samples: usize,
            freq_hz: f64,
            num_frames: usize,
            num_mels: usize,
            features: Vec<Vec<f32>>,
        }
        let go_ref: FbankRef = serde_json::from_str(&json_data).unwrap();

        // Generate the same PCM: 440Hz sine, 6400 samples.
        let n_samples = go_ref.num_samples;
        let mut audio = vec![0u8; n_samples * 2];
        for i in 0..n_samples {
            let t = i as f64 / 16000.0;
            let sample = (16000.0 * (go_ref.freq_hz * 2.0 * PI * t).sin()) as i16;
            audio[2 * i] = sample as u8;
            audio[2 * i + 1] = (sample >> 8) as u8;
        }

        let cfg = FbankConfig::default();
        let rust_features = compute_fbank(&audio, &cfg).expect("fbank should succeed");

        assert_eq!(rust_features.len(), go_ref.num_frames,
            "frame count mismatch: Rust {} vs Go {}", rust_features.len(), go_ref.num_frames);

        let mut max_diff: f32 = 0.0;
        let mut total_checked = 0;
        for (f, (rust_frame, go_frame)) in rust_features.iter().zip(go_ref.features.iter()).enumerate() {
            assert_eq!(rust_frame.len(), go_ref.num_mels);
            for (m, (&rv, &gv)) in rust_frame.iter().zip(go_frame.iter()).enumerate() {
                let diff = (rv - gv).abs();
                if diff > max_diff {
                    max_diff = diff;
                }
                if diff > 1e-4 {
                    panic!("fbank[{f}][{m}] Go={gv} Rust={rv} diff={diff} > 1e-4");
                }
                total_checked += 1;
            }
        }
        eprintln!(
            "fbank cross-lang: {total_checked} values checked, max_diff={max_diff:.2e}"
        );
    }
}
