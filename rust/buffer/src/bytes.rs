//! Convenience functions for creating byte buffers.
//!
//! This module provides pre-configured buffer constructors for common
//! byte buffer sizes, mirroring the Go implementation's convenience functions.

use crate::{BlockBuffer, Buffer, RingBuffer};

// ============================================================================
// Growable Buffer convenience functions
// ============================================================================

/// Creates a 1KB growable buffer for bytes.
pub fn bytes_1kb() -> Buffer<u8> {
    Buffer::with_capacity(1024)
}

/// Creates a 4KB growable buffer for bytes.
pub fn bytes_4kb() -> Buffer<u8> {
    Buffer::with_capacity(4096)
}

/// Creates a 16KB growable buffer for bytes.
pub fn bytes_16kb() -> Buffer<u8> {
    Buffer::with_capacity(16384)
}

/// Creates a 64KB growable buffer for bytes.
pub fn bytes_64kb() -> Buffer<u8> {
    Buffer::with_capacity(65536)
}

/// Creates a 256B growable buffer for bytes.
pub fn bytes_256b() -> Buffer<u8> {
    Buffer::with_capacity(256)
}

/// Creates a default 1KB growable buffer for bytes.
pub fn bytes() -> Buffer<u8> {
    bytes_1kb()
}

// ============================================================================
// BlockBuffer convenience functions
// ============================================================================

/// Creates a 1KB blocking buffer for bytes.
pub fn block_bytes_1kb() -> BlockBuffer<u8> {
    BlockBuffer::new(1024)
}

/// Creates a 4KB blocking buffer for bytes.
pub fn block_bytes_4kb() -> BlockBuffer<u8> {
    BlockBuffer::new(4096)
}

/// Creates a 16KB blocking buffer for bytes.
pub fn block_bytes_16kb() -> BlockBuffer<u8> {
    BlockBuffer::new(16384)
}

/// Creates a 64KB blocking buffer for bytes.
pub fn block_bytes_64kb() -> BlockBuffer<u8> {
    BlockBuffer::new(65536)
}

/// Creates a 256B blocking buffer for bytes.
pub fn block_bytes_256b() -> BlockBuffer<u8> {
    BlockBuffer::new(256)
}

/// Creates a default 1KB blocking buffer for bytes.
pub fn block_bytes() -> BlockBuffer<u8> {
    block_bytes_1kb()
}

// ============================================================================
// RingBuffer convenience functions
// ============================================================================

/// Creates a 1KB ring buffer for bytes.
pub fn ring_bytes_1kb() -> RingBuffer<u8> {
    RingBuffer::new(1024)
}

/// Creates a 4KB ring buffer for bytes.
pub fn ring_bytes_4kb() -> RingBuffer<u8> {
    RingBuffer::new(4096)
}

/// Creates a 16KB ring buffer for bytes.
pub fn ring_bytes_16kb() -> RingBuffer<u8> {
    RingBuffer::new(16384)
}

/// Creates a 64KB ring buffer for bytes.
pub fn ring_bytes_64kb() -> RingBuffer<u8> {
    RingBuffer::new(65536)
}

/// Creates a ring buffer for bytes with the specified capacity.
pub fn ring_bytes(size: usize) -> RingBuffer<u8> {
    RingBuffer::new(size)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bytes_convenience_functions() {
        let b1 = bytes_1kb();
        let b4 = bytes_4kb();
        let b16 = bytes_16kb();
        let b64 = bytes_64kb();

        // Just verify they can be created and used
        b1.write(&[1, 2, 3]).unwrap();
        b4.write(&[1, 2, 3]).unwrap();
        b16.write(&[1, 2, 3]).unwrap();
        b64.write(&[1, 2, 3]).unwrap();
    }

    #[test]
    fn test_block_bytes_convenience_functions() {
        let b1 = block_bytes_1kb();
        let b4 = block_bytes_4kb();

        assert_eq!(b1.capacity(), 1024);
        assert_eq!(b4.capacity(), 4096);

        b1.write(&[1, 2, 3]).unwrap();
        b4.write(&[1, 2, 3]).unwrap();
    }

    #[test]
    fn test_ring_bytes_convenience_functions() {
        let b1 = ring_bytes_1kb();
        let b4 = ring_bytes_4kb();
        let custom = ring_bytes(100);

        assert_eq!(b1.capacity(), 1024);
        assert_eq!(b4.capacity(), 4096);
        assert_eq!(custom.capacity(), 100);

        b1.write(&[1, 2, 3]).unwrap();
        b4.write(&[1, 2, 3]).unwrap();
        custom.write(&[1, 2, 3]).unwrap();
    }
}
