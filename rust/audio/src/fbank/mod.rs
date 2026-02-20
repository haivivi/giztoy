//! Log mel filterbank feature extraction from PCM audio.
//!
//! Standard front-end for speaker recognition models (e.g., 3D-Speaker ERes2Net).
//! Output is a `[T, num_mels]` f32 matrix suitable for inference input.
//!
//! Default parameters match the Kaldi/3D-Speaker convention:
//! - SampleRate: 16000
//! - WindowSize: 400 (25ms)
//! - HopSize: 160 (10ms)
//! - FFTSize: 512
//! - NumMels: 80
//! - LowFreq: 20 Hz
//! - HighFreq: 7600 Hz
//! - PreEmphasis: 0.97

mod fft;
mod mel;

use std::f64::consts::PI;

/// Configuration for mel filterbank extraction.
#[derive(Debug, Clone)]
pub struct Config {
    pub sample_rate: usize,
    pub window_size: usize,
    pub hop_size: usize,
    pub fft_size: usize,
    pub num_mels: usize,
    pub low_freq: f64,
    pub high_freq: f64,
    pub pre_emphasis: f64,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            sample_rate: 16000,
            window_size: 400,
            hop_size: 160,
            fft_size: 512,
            num_mels: 80,
            low_freq: 20.0,
            high_freq: 7600.0,
            pre_emphasis: 0.97,
        }
    }
}

/// Mel filterbank feature extractor.
pub struct Extractor {
    cfg: Config,
    window: Vec<f64>,
    mel_bank: Vec<Vec<f64>>,
}

impl Extractor {
    /// Creates a new extractor with the given config.
    pub fn new(cfg: Config) -> Self {
        let window = mel::hamming_window(cfg.window_size);
        let mel_bank = mel::mel_filter_bank(
            cfg.num_mels, cfg.fft_size, cfg.sample_rate, cfg.low_freq, cfg.high_freq,
        );
        Self { cfg, window, mel_bank }
    }

    /// Extracts log mel filterbank features from normalized f32 PCM samples (range [-1, 1]).
    ///
    /// Returns `[T][num_mels]` where `T = (len(pcm) - window_size) / hop_size + 1`.
    pub fn extract(&self, pcm: &[f32]) -> Vec<Vec<f32>> {
        let cfg = &self.cfg;
        let n = pcm.len();
        if n < cfg.window_size {
            return Vec::new();
        }

        let num_frames = (n - cfg.window_size) / cfg.hop_size + 1;
        let nfft = cfg.fft_size;
        let half_fft = nfft / 2 + 1;

        let mut features = Vec::with_capacity(num_frames);
        let mut frame = vec![0.0f64; nfft];
        let mut real = vec![0.0f64; nfft];
        let mut imag = vec![0.0f64; nfft];

        for t in 0..num_frames {
            let start = t * cfg.hop_size;

            // Pre-emphasis + windowing
            for i in 0..cfg.window_size {
                let mut s = pcm[start + i] as f64;
                if start + i > 0 {
                    s -= cfg.pre_emphasis * pcm[start + i - 1] as f64;
                }
                frame[i] = s * self.window[i];
            }
            // Zero-pad
            for i in cfg.window_size..nfft {
                frame[i] = 0.0;
            }

            // FFT
            real.copy_from_slice(&frame);
            for v in imag.iter_mut() {
                *v = 0.0;
            }
            fft::fft(&mut real, &mut imag);

            // Power spectrum
            let mut power = vec![0.0f64; half_fft];
            for i in 0..half_fft {
                power[i] = real[i] * real[i] + imag[i] * imag[i];
            }

            // Mel filterbank + log
            let mut mel = vec![0.0f32; cfg.num_mels];
            for m in 0..cfg.num_mels {
                let mut sum = 0.0f64;
                for (k, &w) in self.mel_bank[m].iter().enumerate() {
                    sum += w * power[k];
                }
                if sum < 1e-10 {
                    sum = 1e-10;
                }
                mel[m] = sum.ln() as f32;
            }
            features.push(mel);
        }

        features
    }

    /// Extracts features from raw int16 PCM bytes (little-endian).
    pub fn extract_from_int16(&self, pcm_bytes: &[u8]) -> Vec<Vec<f32>> {
        let n = pcm_bytes.len() / 2;
        let mut samples = vec![0.0f32; n];
        for i in 0..n {
            let s = i16::from_le_bytes([pcm_bytes[i * 2], pcm_bytes[i * 2 + 1]]);
            samples[i] = s as f32 / 32768.0;
        }
        self.extract(&samples)
    }
}

/// Applies Cepstral Mean and Variance Normalization in-place.
///
/// For each mel dimension, subtracts the mean and divides by standard deviation
/// across all frames. This removes channel/environment effects for improved
/// speaker verification accuracy.
pub fn cmvn(features: &mut [Vec<f32>]) {
    if features.is_empty() {
        return;
    }
    let num_mels = features[0].len();
    let t = features.len() as f64;

    for m in 0..num_mels {
        let sum: f64 = features.iter().map(|f| f[m] as f64).sum();
        let mean = sum / t;

        let var_sum: f64 = features.iter().map(|f| {
            let d = f[m] as f64 - mean;
            d * d
        }).sum();
        let mut std = (var_sum / t).sqrt();
        if std < 1e-10 {
            std = 1e-10;
        }

        for f in features.iter_mut() {
            f[m] = ((f[m] as f64 - mean) / std) as f32;
        }
    }
}

/// Flattens `[T][num_mels]` to `[T * num_mels]` for ncnn Mat2D input.
pub fn flatten(features: &[Vec<f32>]) -> Vec<f32> {
    if features.is_empty() {
        return Vec::new();
    }
    let cols = features[0].len();
    let mut flat = Vec::with_capacity(features.len() * cols);
    for row in features {
        flat.extend_from_slice(row);
    }
    flat
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_extract_sine_wave() {
        let cfg = Config::default();
        let extractor = Extractor::new(cfg.clone());

        // Generate 1 second of 440Hz sine wave at 16kHz
        let sample_rate = cfg.sample_rate as f64;
        let samples: Vec<f32> = (0..16000)
            .map(|i| (2.0 * PI * 440.0 * i as f64 / sample_rate).sin() as f32)
            .collect();

        let features = extractor.extract(&samples);

        // Expected frames: (16000 - 400) / 160 + 1 = 98
        assert_eq!(features.len(), 98);
        assert_eq!(features[0].len(), 80);

        // Check that values are reasonable (not NaN or Inf)
        for frame in &features {
            for &v in frame {
                assert!(v.is_finite(), "feature value must be finite, got {}", v);
            }
        }
    }

    #[test]
    fn test_extract_from_int16() {
        let cfg = Config::default();
        let extractor = Extractor::new(cfg);

        // 800 samples = 50ms at 16kHz (enough for 1 frame)
        let mut pcm_bytes = vec![0u8; 800 * 2];
        for i in 0..800 {
            let sample = ((2.0 * PI * 440.0 * i as f64 / 16000.0).sin() * 16000.0) as i16;
            let bytes = sample.to_le_bytes();
            pcm_bytes[i * 2] = bytes[0];
            pcm_bytes[i * 2 + 1] = bytes[1];
        }

        let features = extractor.extract_from_int16(&pcm_bytes);
        assert!(!features.is_empty());
        assert_eq!(features[0].len(), 80);
    }

    #[test]
    fn test_cmvn() {
        let mut features = vec![
            vec![1.0f32, 2.0, 3.0],
            vec![4.0, 5.0, 6.0],
            vec![7.0, 8.0, 9.0],
        ];

        cmvn(&mut features);

        // After CMVN, each dimension should have ~zero mean
        for m in 0..3 {
            let mean: f32 = features.iter().map(|f| f[m]).sum::<f32>() / 3.0;
            assert!(mean.abs() < 1e-5, "mean should be ~0, got {}", mean);
        }
    }

    #[test]
    fn test_flatten() {
        let features = vec![
            vec![1.0f32, 2.0],
            vec![3.0, 4.0],
        ];
        let flat = flatten(&features);
        assert_eq!(flat, vec![1.0, 2.0, 3.0, 4.0]);
    }

    #[test]
    fn test_empty_input() {
        let cfg = Config::default();
        let extractor = Extractor::new(cfg);
        assert!(extractor.extract(&[]).is_empty());
        assert!(extractor.extract(&[0.0; 100]).is_empty()); // Too short
    }
}
