//! Ogg sync/decoder for reading Ogg streams.

use std::io::{self, Read};
use super::page::Page;

/// Magic bytes for Ogg page header.
const OGG_MAGIC: &[u8] = b"OggS";

/// Ogg sync state for reading pages.
pub struct OggSync<R: Read> {
    reader: R,
    buffer: Vec<u8>,
}

impl<R: Read> OggSync<R> {
    /// Creates a new Ogg sync state.
    pub fn new(reader: R) -> Self {
        Self {
            reader,
            buffer: Vec::new(),
        }
    }

    /// Reads the next Ogg page.
    pub fn read_page(&mut self) -> io::Result<Option<Page>> {
        // Read header (minimum 27 bytes)
        let mut header = [0u8; 27];
        match self.reader.read_exact(&mut header) {
            Ok(()) => {}
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(None),
            Err(e) => return Err(e),
        }

        // Verify magic
        if &header[0..4] != OGG_MAGIC {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                "invalid Ogg magic",
            ));
        }

        let version = header[4];
        let header_type = header[5];
        let granule_position = i64::from_le_bytes(header[6..14].try_into().unwrap());
        let serial = u32::from_le_bytes(header[14..18].try_into().unwrap());
        let sequence = u32::from_le_bytes(header[18..22].try_into().unwrap());
        let checksum = u32::from_le_bytes(header[22..26].try_into().unwrap());
        let segments = header[26];

        // Read segment table
        let mut segment_table = vec![0u8; segments as usize];
        self.reader.read_exact(&mut segment_table)?;

        // Calculate body size
        let body_size: usize = segment_table.iter().map(|&s| s as usize).sum();

        // Read body
        let mut body = vec![0u8; body_size];
        self.reader.read_exact(&mut body)?;

        Ok(Some(Page {
            version,
            header_type,
            granule_position,
            serial,
            sequence,
            checksum,
            segments,
            segment_table,
            body,
        }))
    }

    /// Returns the inner reader.
    pub fn into_inner(self) -> R {
        self.reader
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use super::super::encoder::OggEncoder;
    use std::io::Cursor;

    fn create_test_ogg_data() -> Vec<u8> {
        let mut buf = Vec::new();
        {
            let cursor = Cursor::new(&mut buf);
            let mut encoder = OggEncoder::new(cursor, 12345);
            
            // Write a BOS packet
            let packet = vec![0u8; 100];
            encoder.write_packet(&packet, 960, true, false).unwrap();
            
            // Write another packet
            encoder.write_packet(&packet, 960, false, false).unwrap();
            
            // Write EOS packet
            encoder.write_packet(&packet, 960, false, true).unwrap();
        }
        buf
    }

    #[test]
    fn test_ogg_sync_new() {
        let data = create_test_ogg_data();
        let cursor = Cursor::new(data);
        let sync = OggSync::new(cursor);
        assert!(sync.buffer.is_empty());
    }

    #[test]
    fn test_ogg_sync_read_page() {
        let data = create_test_ogg_data();
        let cursor = Cursor::new(data);
        let mut sync = OggSync::new(cursor);
        
        // Read first page (BOS)
        let page = sync.read_page().unwrap();
        assert!(page.is_some());
        let p = page.unwrap();
        assert!(p.is_bos());
        assert_eq!(p.serial, 12345);
    }

    #[test]
    fn test_ogg_sync_read_multiple_pages() {
        let data = create_test_ogg_data();
        let cursor = Cursor::new(data);
        let mut sync = OggSync::new(cursor);
        
        // Read all pages
        let mut count = 0;
        let mut found_bos = false;
        let mut found_eos = false;
        
        while let Some(page) = sync.read_page().unwrap() {
            count += 1;
            if page.is_bos() {
                found_bos = true;
            }
            if page.is_eos() {
                found_eos = true;
            }
        }
        
        assert_eq!(count, 3);
        assert!(found_bos);
        assert!(found_eos);
    }

    #[test]
    fn test_ogg_sync_empty_input() {
        let cursor = Cursor::new(Vec::<u8>::new());
        let mut sync = OggSync::new(cursor);
        
        let page = sync.read_page().unwrap();
        assert!(page.is_none());
    }

    #[test]
    fn test_ogg_sync_invalid_magic() {
        let data = vec![0u8; 100]; // Invalid data, no OggS magic
        let cursor = Cursor::new(data);
        let mut sync = OggSync::new(cursor);
        
        let result = sync.read_page();
        assert!(result.is_err());
    }

    #[test]
    fn test_ogg_sync_into_inner() {
        let data = create_test_ogg_data();
        let cursor = Cursor::new(data.clone());
        let sync = OggSync::new(cursor);
        
        let inner = sync.into_inner();
        assert_eq!(inner.into_inner(), data);
    }

    #[test]
    fn test_ogg_sync_page_fields() {
        let data = create_test_ogg_data();
        let cursor = Cursor::new(data);
        let mut sync = OggSync::new(cursor);
        
        let page = sync.read_page().unwrap().unwrap();
        
        assert_eq!(page.version, 0);
        assert_eq!(page.serial, 12345);
        assert!(page.granule_position >= 0);
        assert!(!page.body.is_empty());
    }

    #[test]
    fn test_ogg_sync_sequence_numbers() {
        let data = create_test_ogg_data();
        let cursor = Cursor::new(data);
        let mut sync = OggSync::new(cursor);
        
        let mut sequences = Vec::new();
        while let Some(page) = sync.read_page().unwrap() {
            sequences.push(page.sequence);
        }
        
        // Sequences should be 0, 1, 2
        assert_eq!(sequences.len(), 3);
        assert_eq!(sequences[0], 0);
        assert_eq!(sequences[1], 1);
        assert_eq!(sequences[2], 2);
    }

    #[test]
    fn test_roundtrip_encode_decode() {
        // Create some test data
        let mut buf = Vec::new();
        {
            let cursor = Cursor::new(&mut buf);
            let mut encoder = OggEncoder::new(cursor, 54321);
            
            let test_data = b"Hello, Ogg world!";
            encoder.write_packet(test_data, 100, true, false).unwrap();
            encoder.write_packet(test_data, 100, false, true).unwrap();
        }
        
        // Now read it back
        let cursor = Cursor::new(buf);
        let mut sync = OggSync::new(cursor);
        
        let page1 = sync.read_page().unwrap().unwrap();
        assert!(page1.is_bos());
        assert_eq!(page1.serial, 54321);
        
        let page2 = sync.read_page().unwrap().unwrap();
        assert!(page2.is_eos());
        
        // No more pages
        assert!(sync.read_page().unwrap().is_none());
    }
}
