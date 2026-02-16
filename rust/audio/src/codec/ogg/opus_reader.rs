//! Read Opus packets from an OGG container.

use std::io::{self, Read};
use super::opus_writer::OpusPacket;

const PAGE_HEADER_SIGNATURE: &[u8] = b"OggS";
const PAGE_HEADER_SIZE: usize = 27;

/// Reads Opus packets from an OGG stream.
///
/// Yields `OpusPacket` instances for each audio packet found,
/// skipping header pages (OpusHead, OpusTags).
pub struct OpusPacketReader<R: Read> {
    reader: R,
    buf: Vec<u8>,
}

impl<R: Read> OpusPacketReader<R> {
    /// Creates a new Opus packet reader.
    pub fn new(reader: R) -> Self {
        Self {
            reader,
            buf: Vec::new(),
        }
    }

    /// Reads the next Opus packet from the OGG stream.
    ///
    /// Returns `Ok(None)` at end of stream.
    /// Skips header pages (OpusHead, OpusTags).
    pub fn read_packet(&mut self) -> io::Result<Option<OpusPacket>> {
        loop {
            // Read page header
            let mut header = [0u8; PAGE_HEADER_SIZE];
            match self.reader.read_exact(&mut header) {
                Ok(()) => {}
                Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(None),
                Err(e) => return Err(e),
            }

            // Verify signature
            if &header[..4] != PAGE_HEADER_SIGNATURE {
                return Err(io::Error::new(io::ErrorKind::InvalidData, "invalid OGG page signature"));
            }

            let header_type = header[5];
            let granule = i64::from_le_bytes(header[6..14].try_into().unwrap());
            let serial_no = i32::from_le_bytes(header[14..18].try_into().unwrap());
            let n_segments = header[26] as usize;

            // Read segment table
            let mut segment_table = vec![0u8; n_segments];
            self.reader.read_exact(&mut segment_table)?;

            // Calculate payload size
            let payload_size: usize = segment_table.iter().map(|&s| s as usize).sum();

            // Read payload
            let mut payload = vec![0u8; payload_size];
            self.reader.read_exact(&mut payload)?;

            // Skip BOS pages (headers)
            if header_type & 0x02 != 0 {
                continue;
            }

            // Skip header-like payloads (OpusHead, OpusTags)
            if payload.len() >= 8 && (&payload[..8] == b"OpusHead" || &payload[..8] == b"OpusTags") {
                continue;
            }

            // Skip empty EOS pages
            if payload.is_empty() {
                if header_type & 0x04 != 0 {
                    return Ok(None); // EOS
                }
                continue;
            }

            return Ok(Some(OpusPacket {
                data: payload,
                granule,
                serial_no,
            }));
        }
    }
}

/// Returns an iterator over Opus packets in an OGG stream.
pub fn read_opus_packets<R: Read>(reader: R) -> OpusPacketIter<R> {
    OpusPacketIter {
        reader: OpusPacketReader::new(reader),
        done: false,
    }
}

/// Iterator over Opus packets in an OGG stream.
pub struct OpusPacketIter<R: Read> {
    reader: OpusPacketReader<R>,
    done: bool,
}

impl<R: Read> Iterator for OpusPacketIter<R> {
    type Item = io::Result<OpusPacket>;

    fn next(&mut self) -> Option<Self::Item> {
        if self.done {
            return None;
        }
        match self.reader.read_packet() {
            Ok(Some(packet)) => Some(Ok(packet)),
            Ok(None) => {
                self.done = true;
                None
            }
            Err(e) => {
                self.done = true;
                Some(Err(e))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use super::super::opus_writer::OpusWriter;

    #[test]
    fn test_write_read_roundtrip() {
        // Write some opus frames
        let mut ogg_data = Vec::new();
        {
            let mut writer = OpusWriter::new(&mut ogg_data, 48000, 1).unwrap();
            writer.write_frame(&[0xFC, 0x01, 0x02, 0x03], 960).unwrap();
            writer.write_frame(&[0xFC, 0x04, 0x05, 0x06], 960).unwrap();
            writer.write_frame(&[0xFC, 0x07, 0x08, 0x09], 960).unwrap();
            writer.close().unwrap();
        }

        // Read them back
        let packets: Vec<OpusPacket> = read_opus_packets(io::Cursor::new(&ogg_data))
            .collect::<io::Result<Vec<_>>>()
            .unwrap();

        assert_eq!(packets.len(), 3);
        assert_eq!(packets[0].data, vec![0xFC, 0x01, 0x02, 0x03]);
        assert_eq!(packets[1].data, vec![0xFC, 0x04, 0x05, 0x06]);
        assert_eq!(packets[2].data, vec![0xFC, 0x07, 0x08, 0x09]);
        assert_eq!(packets[2].granule, 2880); // 3 * 960
    }

    #[test]
    fn test_empty_stream() {
        let packets: Vec<_> = read_opus_packets(io::Cursor::new(&[]))
            .collect::<io::Result<Vec<_>>>()
            .unwrap();
        assert!(packets.is_empty());
    }
}
