//! MQTT packet encoding/decoding utilities.
//!
//! This module provides low-level encoding and decoding functions
//! for MQTT protocol primitives.

#[cfg(feature = "alloc")]
use alloc::string::String;
#[cfg(feature = "alloc")]
use alloc::vec::Vec;

use crate::error::{Error, Result};
use crate::types::{FixedHeader, PacketType};

/// Read a variable-length integer (remaining length encoding).
///
/// Returns `(value, bytes_consumed)` or `None` if incomplete.
pub fn read_variable_int(buf: &[u8]) -> Option<(u32, usize)> {
    let mut multiplier = 1u32;
    let mut value = 0u32;

    for (i, &byte) in buf.iter().enumerate() {
        value += (byte & 0x7F) as u32 * multiplier;

        if multiplier > 128 * 128 * 128 {
            return None; // Malformed
        }

        if byte & 0x80 == 0 {
            return Some((value, i + 1));
        }

        multiplier *= 128;
    }

    None // Incomplete
}

/// Write a variable-length integer.
///
/// Returns the number of bytes written, or `None` if buffer too small.
pub fn write_variable_int(buf: &mut [u8], mut value: u32) -> Option<usize> {
    let mut i = 0;

    loop {
        if i >= buf.len() {
            return None;
        }

        let mut byte = (value % 128) as u8;
        value /= 128;

        if value > 0 {
            byte |= 0x80;
        }

        buf[i] = byte;
        i += 1;

        if value == 0 {
            break;
        }
    }

    Some(i)
}

/// Calculate the number of bytes needed for a variable-length integer.
pub const fn variable_int_len(value: u32) -> usize {
    if value < 128 {
        1
    } else if value < 128 * 128 {
        2
    } else if value < 128 * 128 * 128 {
        3
    } else {
        4
    }
}

/// Read a 2-byte big-endian u16.
pub fn read_u16(buf: &[u8]) -> Option<u16> {
    if buf.len() < 2 {
        return None;
    }
    Some(u16::from_be_bytes([buf[0], buf[1]]))
}

/// Write a 2-byte big-endian u16.
pub fn write_u16(buf: &mut [u8], value: u16) -> Option<()> {
    if buf.len() < 2 {
        return None;
    }
    let bytes = value.to_be_bytes();
    buf[0] = bytes[0];
    buf[1] = bytes[1];
    Some(())
}

/// Read a 4-byte big-endian u32.
pub fn read_u32(buf: &[u8]) -> Option<u32> {
    if buf.len() < 4 {
        return None;
    }
    Some(u32::from_be_bytes([buf[0], buf[1], buf[2], buf[3]]))
}

/// Write a 4-byte big-endian u32.
pub fn write_u32(buf: &mut [u8], value: u32) -> Option<()> {
    if buf.len() < 4 {
        return None;
    }
    let bytes = value.to_be_bytes();
    buf[..4].copy_from_slice(&bytes);
    Some(())
}

/// Read a UTF-8 string (2-byte length prefix + data).
#[cfg(feature = "alloc")]
pub fn read_string(buf: &[u8]) -> Result<(String, usize)> {
    let len = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })? as usize;

    if buf.len() < 2 + len {
        return Err(Error::Incomplete { needed: 2 + len - buf.len() });
    }

    let s = core::str::from_utf8(&buf[2..2 + len])
        .map_err(|_| Error::InvalidUtf8)?
        .to_string();

    Ok((s, 2 + len))
}

/// Read a UTF-8 string as a slice (2-byte length prefix + data).
pub fn read_string_slice(buf: &[u8]) -> Result<(&str, usize)> {
    let len = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })? as usize;

    if buf.len() < 2 + len {
        return Err(Error::Incomplete { needed: 2 + len - buf.len() });
    }

    let s = core::str::from_utf8(&buf[2..2 + len])
        .map_err(|_| Error::InvalidUtf8)?;

    Ok((s, 2 + len))
}

/// Write a UTF-8 string (2-byte length prefix + data).
pub fn write_string(buf: &mut [u8], s: &str) -> Option<usize> {
    let bytes = s.as_bytes();
    let len = bytes.len();

    if len > u16::MAX as usize || buf.len() < 2 + len {
        return None;
    }

    write_u16(buf, len as u16)?;
    buf[2..2 + len].copy_from_slice(bytes);

    Some(2 + len)
}

/// Read binary data (2-byte length prefix + data).
#[cfg(feature = "alloc")]
pub fn read_binary(buf: &[u8]) -> Result<(Vec<u8>, usize)> {
    let len = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })? as usize;

    if buf.len() < 2 + len {
        return Err(Error::Incomplete { needed: 2 + len - buf.len() });
    }

    Ok((buf[2..2 + len].to_vec(), 2 + len))
}

/// Read binary data as a slice (2-byte length prefix + data).
pub fn read_binary_slice(buf: &[u8]) -> Result<(&[u8], usize)> {
    let len = read_u16(buf).ok_or(Error::Incomplete { needed: 2 })? as usize;

    if buf.len() < 2 + len {
        return Err(Error::Incomplete { needed: 2 + len - buf.len() });
    }

    Ok((&buf[2..2 + len], 2 + len))
}

/// Write binary data (2-byte length prefix + data).
pub fn write_binary(buf: &mut [u8], data: &[u8]) -> Option<usize> {
    let len = data.len();

    if len > u16::MAX as usize || buf.len() < 2 + len {
        return None;
    }

    write_u16(buf, len as u16)?;
    buf[2..2 + len].copy_from_slice(data);

    Some(2 + len)
}

/// Parse a fixed header from buffer.
pub fn read_fixed_header(buf: &[u8]) -> Result<FixedHeader> {
    if buf.is_empty() {
        return Err(Error::Incomplete { needed: 1 });
    }

    let first_byte = buf[0];
    let packet_type_byte = first_byte >> 4;
    let flags = first_byte & 0x0F;

    let packet_type = PacketType::from_u8(packet_type_byte)
        .ok_or(Error::InvalidPacketType(packet_type_byte))?;

    let (remaining_length, var_len) = read_variable_int(&buf[1..])
        .ok_or(Error::Incomplete { needed: 1 })?;

    Ok(FixedHeader {
        packet_type,
        flags,
        remaining_length,
        header_length: 1 + var_len,
    })
}

/// Write a fixed header to buffer.
pub fn write_fixed_header(buf: &mut [u8], packet_type: PacketType, flags: u8, remaining_length: u32) -> Option<usize> {
    if buf.is_empty() {
        return None;
    }

    buf[0] = ((packet_type as u8) << 4) | (flags & 0x0F);
    let var_len = write_variable_int(&mut buf[1..], remaining_length)?;

    Some(1 + var_len)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_variable_int() {
        let mut buf = [0u8; 4];

        // Test encoding and decoding
        for value in [0, 1, 127, 128, 16383, 16384, 2097151, 2097152, 268435455] {
            let written = write_variable_int(&mut buf, value).unwrap();
            let (decoded, consumed) = read_variable_int(&buf).unwrap();
            assert_eq!(decoded, value);
            assert_eq!(written, consumed);
            assert_eq!(written, variable_int_len(value));
        }
    }

    #[test]
    fn test_u16() {
        let mut buf = [0u8; 2];
        write_u16(&mut buf, 0x1234).unwrap();
        assert_eq!(read_u16(&buf).unwrap(), 0x1234);
    }

    #[test]
    fn test_string() {
        let mut buf = [0u8; 20];
        let len = write_string(&mut buf, "hello").unwrap();
        assert_eq!(len, 7); // 2 + 5

        let (s, consumed) = read_string_slice(&buf).unwrap();
        assert_eq!(s, "hello");
        assert_eq!(consumed, 7);
    }
}
