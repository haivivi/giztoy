//! Mel-scale utilities and filterbank generation.

use std::f64::consts::PI;

/// Generates a Hamming window of the given length.
pub fn hamming_window(n: usize) -> Vec<f64> {
    if n <= 1 {
        return vec![1.0; n];
    }
    (0..n)
        .map(|i| 0.54 - 0.46 * (2.0 * PI * i as f64 / (n - 1) as f64).cos())
        .collect()
}

/// Converts frequency in Hz to mel scale.
fn hz_to_mel(hz: f64) -> f64 {
    2595.0 * (1.0 + hz / 700.0).log10()
}

/// Converts mel scale frequency back to Hz.
fn mel_to_hz(mel: f64) -> f64 {
    700.0 * (10.0_f64.powf(mel / 2595.0) - 1.0)
}

/// Creates the mel filterbank matrix.
///
/// Returns `[num_mels][half_fft]` where `half_fft = fft_size / 2 + 1`.
pub fn mel_filter_bank(
    num_mels: usize,
    fft_size: usize,
    sample_rate: usize,
    low_freq: f64,
    high_freq: f64,
) -> Vec<Vec<f64>> {
    let half_fft = fft_size / 2 + 1;
    let low_mel = hz_to_mel(low_freq);
    let high_mel = hz_to_mel(high_freq);

    // num_mels + 2 equally spaced mel points
    let step = (high_mel - low_mel) / (num_mels + 1) as f64;
    let mel_points: Vec<f64> = (0..num_mels + 2)
        .map(|i| low_mel + i as f64 * step)
        .collect();

    // Convert mel points to FFT bin indices
    let mut bins: Vec<usize> = mel_points
        .iter()
        .map(|&m| {
            let hz = mel_to_hz(m);
            let bin = (hz * fft_size as f64 / sample_rate as f64).round() as usize;
            bin.min(half_fft - 1)
        })
        .collect();

    // Ensure each filter has at least 1 bin width
    for i in 1..bins.len() {
        if bins[i] <= bins[i - 1] {
            bins[i] = bins[i - 1] + 1;
        }
    }

    // Create triangular filters
    let mut bank = Vec::with_capacity(num_mels);
    for m in 0..num_mels {
        let mut filter = vec![0.0f64; half_fft];
        let left = bins[m];
        let center = bins[m + 1];
        let right = bins[m + 2];

        for k in left..center.min(half_fft) {
            if center != left {
                filter[k] = (k - left) as f64 / (center - left) as f64;
            }
        }
        for k in center..=right.min(half_fft - 1) {
            if right != center {
                filter[k] = (right - k) as f64 / (right - center) as f64;
            }
        }
        bank.push(filter);
    }
    bank
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hamming_window() {
        let w = hamming_window(400);
        assert_eq!(w.len(), 400);
        // Hamming window should be symmetric
        for i in 0..200 {
            assert!((w[i] - w[399 - i]).abs() < 1e-10);
        }
        // First and last values should be ~0.08
        assert!((w[0] - 0.08).abs() < 0.01);
        // Center should be ~1.0
        assert!((w[199] - 1.0).abs() < 0.01);
    }

    #[test]
    fn test_hz_mel_roundtrip() {
        for &hz in &[0.0, 100.0, 440.0, 1000.0, 4000.0, 8000.0] {
            let mel = hz_to_mel(hz);
            let back = mel_to_hz(mel);
            assert!((hz - back).abs() < 1e-6, "roundtrip failed for {} Hz", hz);
        }
    }

    #[test]
    fn test_mel_filter_bank_shape() {
        let bank = mel_filter_bank(80, 512, 16000, 20.0, 7600.0);
        assert_eq!(bank.len(), 80);
        assert_eq!(bank[0].len(), 257); // 512/2 + 1

        // Each filter should have non-negative values
        for filter in &bank {
            for &v in filter {
                assert!(v >= 0.0);
            }
        }
    }
}
