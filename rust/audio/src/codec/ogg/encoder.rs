//! Ogg encoder/writer.

use std::io::{self, Write};
use super::page::{Page, flags};
use super::stream::StreamState;

/// CRC lookup table for Ogg.
static CRC_TABLE: [u32; 256] = {
    let mut table = [0u32; 256];
    let mut i = 0;
    while i < 256 {
        let mut r = (i as u32) << 24;
        let mut j = 0;
        while j < 8 {
            if r & 0x80000000 != 0 {
                r = (r << 1) ^ 0x04c11db7;
            } else {
                r <<= 1;
            }
            j += 1;
        }
        table[i] = r;
        i += 1;
    }
    table
};

/// Calculates CRC for Ogg page.
fn crc32(data: &[u8]) -> u32 {
    let mut crc = 0u32;
    for &byte in data {
        crc = (crc << 8) ^ CRC_TABLE[((crc >> 24) as u8 ^ byte) as usize];
    }
    crc
}

/// Ogg encoder for writing Ogg streams.
pub struct OggEncoder<W: Write> {
    writer: W,
    state: StreamState,
}

impl<W: Write> OggEncoder<W> {
    /// Creates a new Ogg encoder.
    pub fn new(writer: W, serial: u32) -> Self {
        Self {
            writer,
            state: StreamState::new(serial),
        }
    }

    /// Writes an Ogg page.
    pub fn write_page(&mut self, page: &Page) -> io::Result<()> {
        // Build header
        let mut header = Vec::with_capacity(27 + page.segments as usize);
        
        // Capture pattern "OggS"
        header.extend_from_slice(b"OggS");
        // Version
        header.push(page.version);
        // Header type
        header.push(page.header_type);
        // Granule position (little-endian)
        header.extend_from_slice(&page.granule_position.to_le_bytes());
        // Serial number
        header.extend_from_slice(&page.serial.to_le_bytes());
        // Sequence number
        header.extend_from_slice(&page.sequence.to_le_bytes());
        // CRC (placeholder, will be filled later)
        header.extend_from_slice(&[0u8; 4]);
        // Number of segments
        header.push(page.segments);
        // Segment table
        header.extend_from_slice(&page.segment_table);

        // Calculate CRC
        let mut crc_data = header.clone();
        crc_data.extend_from_slice(&page.body);
        let crc = crc32(&crc_data);
        
        // Update CRC in header
        header[22..26].copy_from_slice(&crc.to_le_bytes());

        // Write header and body
        self.writer.write_all(&header)?;
        self.writer.write_all(&page.body)?;

        Ok(())
    }

    /// Writes a packet to the stream.
    pub fn write_packet(&mut self, data: &[u8], granule_increment: i64, is_bos: bool, is_eos: bool) -> io::Result<()> {
        // Build segment table
        let mut segment_table = Vec::new();
        let mut remaining = data.len();
        while remaining >= 255 {
            segment_table.push(255u8);
            remaining -= 255;
        }
        segment_table.push(remaining as u8);

        // Update state
        self.state.set_granule_position(self.state.granule_position() + granule_increment);
        
        // Build header type
        let mut header_type = 0u8;
        if is_bos {
            header_type |= flags::BOS;
        }
        if is_eos {
            header_type |= flags::EOS;
        }

        let page = Page {
            version: 0,
            header_type,
            granule_position: self.state.granule_position(),
            serial: self.state.serial(),
            sequence: self.state.sequence(),
            checksum: 0, // Will be calculated
            segments: segment_table.len() as u8,
            segment_table,
            body: data.to_vec(),
        };

        self.write_page(&page)?;
        self.state.increment_sequence();

        Ok(())
    }

    /// Flushes the writer.
    pub fn flush(&mut self) -> io::Result<()> {
        self.writer.flush()
    }

    /// Returns the inner writer.
    pub fn into_inner(self) -> W {
        self.writer
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn test_crc32() {
        // Test with known value
        let data = b"OggS";
        let crc = crc32(data);
        assert!(crc != 0); // Just verify it computes something
    }

    #[test]
    fn test_crc32_empty() {
        let crc = crc32(&[]);
        assert_eq!(crc, 0);
    }

    #[test]
    fn test_crc32_consistency() {
        let data = b"Hello, World!";
        let crc1 = crc32(data);
        let crc2 = crc32(data);
        assert_eq!(crc1, crc2);
    }

    #[test]
    fn test_ogg_encoder_new() {
        let buf = Vec::new();
        let encoder = OggEncoder::new(buf, 12345);
        assert_eq!(encoder.state.serial(), 12345);
    }

    #[test]
    fn test_ogg_encoder_write_packet_bos() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        let packet = vec![0u8; 100];
        let result = encoder.write_packet(&packet, 960, true, false);
        assert!(result.is_ok());
        
        // Check that state was updated
        assert_eq!(encoder.state.sequence(), 1);
        assert_eq!(encoder.state.granule_position(), 960);
    }

    #[test]
    fn test_ogg_encoder_write_packet_eos() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        let packet = vec![0u8; 100];
        let result = encoder.write_packet(&packet, 960, false, true);
        assert!(result.is_ok());
    }

    #[test]
    fn test_ogg_encoder_write_multiple_packets() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        let packet = vec![0u8; 100];
        
        // First packet (BOS)
        encoder.write_packet(&packet, 960, true, false).unwrap();
        assert_eq!(encoder.state.sequence(), 1);
        
        // Second packet
        encoder.write_packet(&packet, 960, false, false).unwrap();
        assert_eq!(encoder.state.sequence(), 2);
        
        // Third packet (EOS)
        encoder.write_packet(&packet, 960, false, true).unwrap();
        assert_eq!(encoder.state.sequence(), 3);
        
        assert_eq!(encoder.state.granule_position(), 2880);
    }

    #[test]
    fn test_ogg_encoder_large_packet() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        // Packet larger than 255 bytes (needs multiple segments)
        let packet = vec![0u8; 500];
        let result = encoder.write_packet(&packet, 960, true, false);
        assert!(result.is_ok());
    }

    #[test]
    fn test_ogg_encoder_exact_255_packet() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        // Packet exactly 255 bytes
        let packet = vec![0u8; 255];
        let result = encoder.write_packet(&packet, 960, true, false);
        assert!(result.is_ok());
    }

    #[test]
    fn test_ogg_encoder_multiple_of_255_packet() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        // Packet that's a multiple of 255 bytes
        let packet = vec![0u8; 510];
        let result = encoder.write_packet(&packet, 960, true, false);
        assert!(result.is_ok());
    }

    #[test]
    fn test_ogg_encoder_write_page() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        let page = Page {
            version: 0,
            header_type: flags::BOS,
            granule_position: 0,
            serial: 12345,
            sequence: 0,
            checksum: 0,
            segments: 1,
            segment_table: vec![10],
            body: vec![0; 10],
        };
        
        let result = encoder.write_page(&page);
        assert!(result.is_ok());
    }

    #[test]
    fn test_ogg_encoder_flush() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        let result = encoder.flush();
        assert!(result.is_ok());
    }

    #[test]
    fn test_ogg_encoder_into_inner() {
        let buf = Vec::new();
        let encoder = OggEncoder::new(buf, 12345);
        let inner = encoder.into_inner();
        assert!(inner.is_empty());
    }

    #[test]
    fn test_ogg_encoder_output_has_magic() {
        let buf = Vec::new();
        let mut encoder = OggEncoder::new(buf, 12345);
        
        let packet = vec![0u8; 10];
        encoder.write_packet(&packet, 960, true, false).unwrap();
        
        let output = encoder.into_inner();
        assert!(output.len() >= 4);
        assert_eq!(&output[0..4], b"OggS");
    }

    #[test]
    fn test_ogg_encoder_empty_packet() {
        let mut buf = Vec::new();
        let cursor = Cursor::new(&mut buf);
        let mut encoder = OggEncoder::new(cursor, 12345);
        
        let packet: Vec<u8> = vec![];
        let result = encoder.write_packet(&packet, 960, true, false);
        assert!(result.is_ok());
    }
}
