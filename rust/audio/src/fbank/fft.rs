//! In-place radix-2 Cooley-Tukey FFT.

use std::f64::consts::PI;

/// Performs an in-place radix-2 Cooley-Tukey FFT.
/// `real` and `imag` must have the same power-of-2 length.
pub fn fft(real: &mut [f64], imag: &mut [f64]) {
    fft_core(real, imag);
}

/// Performs an in-place inverse FFT.
/// `real` and `imag` must have the same power-of-2 length.
#[allow(dead_code)]
pub fn ifft(real: &mut [f64], imag: &mut [f64]) {
    let n = real.len();
    for v in imag.iter_mut() {
        *v = -*v;
    }
    fft_core(real, imag);
    let scale = 1.0 / n as f64;
    for v in real.iter_mut() {
        *v *= scale;
    }
    for v in imag.iter_mut() {
        *v *= -scale;
    }
}

fn fft_core(real: &mut [f64], imag: &mut [f64]) {
    let n = real.len();
    if n <= 1 {
        return;
    }

    // Bit-reversal permutation
    let mut j = 0usize;
    for i in 0..n - 1 {
        if i < j {
            real.swap(i, j);
            imag.swap(i, j);
        }
        let mut k = n >> 1;
        while k <= j {
            j -= k;
            k >>= 1;
        }
        j += k;
    }

    // Cooley-Tukey butterfly
    let mut size = 2;
    while size <= n {
        let half = size >> 1;
        let angle = -2.0 * PI / size as f64;
        let w_r = angle.cos();
        let w_i = angle.sin();

        let mut start = 0;
        while start < n {
            let (mut t_r, mut t_i) = (1.0, 0.0);
            for k in 0..half {
                let u = start + k;
                let v = u + half;

                let tmp_r = t_r * real[v] - t_i * imag[v];
                let tmp_i = t_r * imag[v] + t_i * real[v];

                real[v] = real[u] - tmp_r;
                imag[v] = imag[u] - tmp_i;
                real[u] += tmp_r;
                imag[u] += tmp_i;

                let new_t_r = t_r * w_r - t_i * w_i;
                let new_t_i = t_r * w_i + t_i * w_r;
                t_r = new_t_r;
                t_i = new_t_i;
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
    fn test_fft_impulse() {
        // FFT of unit impulse should be all 1s
        let mut real = vec![0.0; 8];
        let mut imag = vec![0.0; 8];
        real[0] = 1.0;

        fft(&mut real, &mut imag);

        for &v in &real {
            assert!((v - 1.0).abs() < 1e-10);
        }
        for &v in &imag {
            assert!(v.abs() < 1e-10);
        }
    }

    #[test]
    fn test_fft_ifft_roundtrip() {
        let original = vec![1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0];
        let mut real = original.clone();
        let mut imag = vec![0.0; 8];

        fft(&mut real, &mut imag);
        ifft(&mut real, &mut imag);

        for (a, b) in real.iter().zip(original.iter()) {
            assert!((a - b).abs() < 1e-10, "FFT-IFFT roundtrip failed: {} != {}", a, b);
        }
    }
}
