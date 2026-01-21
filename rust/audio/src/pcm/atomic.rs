//! Atomic floating-point operations.

use std::sync::atomic::{AtomicU32, Ordering};

/// Provides atomic operations for f32 values.
///
/// Uses atomic u32 operations internally by bit-casting the f32.
#[derive(Debug)]
pub struct AtomicF32 {
    bits: AtomicU32,
}

impl AtomicF32 {
    /// Creates a new AtomicF32 with the given initial value.
    pub fn new(val: f32) -> Self {
        Self {
            bits: AtomicU32::new(val.to_bits()),
        }
    }

    /// Atomically loads and returns the f32 value.
    #[inline]
    pub fn load(&self, ordering: Ordering) -> f32 {
        f32::from_bits(self.bits.load(ordering))
    }

    /// Atomically stores the given f32 value.
    #[inline]
    pub fn store(&self, val: f32, ordering: Ordering) {
        self.bits.store(val.to_bits(), ordering);
    }

    /// Atomically swaps the value, returning the previous value.
    #[inline]
    pub fn swap(&self, val: f32, ordering: Ordering) -> f32 {
        f32::from_bits(self.bits.swap(val.to_bits(), ordering))
    }
}

impl Default for AtomicF32 {
    fn default() -> Self {
        Self::new(0.0)
    }
}

impl Clone for AtomicF32 {
    fn clone(&self) -> Self {
        Self::new(self.load(Ordering::SeqCst))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_atomic_f32() {
        let af = AtomicF32::new(1.5);
        assert_eq!(af.load(Ordering::SeqCst), 1.5);

        af.store(2.5, Ordering::SeqCst);
        assert_eq!(af.load(Ordering::SeqCst), 2.5);

        let old = af.swap(3.5, Ordering::SeqCst);
        assert_eq!(old, 2.5);
        assert_eq!(af.load(Ordering::SeqCst), 3.5);
    }
}
