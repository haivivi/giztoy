//! Read Opus packets from an OGG container.
//!
//! Correctly parses OGG page segment tables to extract packet boundaries,
//! supporting pages that contain multiple packets (per RFC 3533).

use std::collections::{HashSet, VecDeque};
use std::io::{self, Read};
use super::opus_writer::OpusPacket;

const PAGE_HEADER_SIGNATURE: &[u8] = b"OggS";
const PAGE_HEADER_SIZE: usize = 27;

/// Reads Opus packets from an OGG stream.
///
/// Correctly handles:
/// - Multiple packets per page (parsed from segment table lacing values)
/// - Packets spanning multiple pages (continuation flag)
/// - Multi-stream OGG (terminates only when all streams have EOS)
/// - Header pages (OpusHead, OpusTags) are skipped
pub struct OpusPacketReader<R: Read> {
    reader: R,
    known_streams: HashSet<i32>,
    eos_streams: HashSet<i32>,
    /// Buffered packets extracted from current page but not yet returned.
    pending: VecDeque<OpusPacket>,
    /// Partial packet data carried over from a page that ended with lacing value 255.
    continued_packet: Vec<u8>,
    continued_serial: i32,
    all_eos: bool,
}

impl<R: Read> OpusPacketReader<R> {
    /// Creates a new Opus packet reader.
    pub fn new(reader: R) -> Self {
        Self {
            reader,
            known_streams: HashSet::new(),
            eos_streams: HashSet::new(),
            pending: VecDeque::new(),
            continued_packet: Vec::new(),
            continued_serial: 0,
            all_eos: false,
        }
    }

    /// Reads the next Opus packet from the OGG stream.
    ///
    /// Returns `Ok(None)` at end of stream (all streams have EOS).
    /// Skips header pages (OpusHead, OpusTags).
    pub fn read_packet(&mut self) -> io::Result<Option<OpusPacket>> {
        loop {
            // Return buffered packets first
            if let Some(pkt) = self.pending.pop_front() {
                return Ok(Some(pkt));
            }

            if self.all_eos {
                return Ok(None);
            }

            // Read next page and extract packets from it
            if !self.read_page()? {
                return Ok(None); // EOF
            }
        }
    }

    /// Reads one OGG page, extracts packets into self.pending.
    /// Returns false on physical EOF.
    fn read_page(&mut self) -> io::Result<bool> {
        // Read page header
        let mut header = [0u8; PAGE_HEADER_SIZE];
        match self.reader.read_exact(&mut header) {
            Ok(()) => {}
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(false),
            Err(e) => return Err(e),
        }

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

        // Read full page payload
        let payload_size: usize = segment_table.iter().map(|&s| s as usize).sum();
        let mut payload = vec![0u8; payload_size];
        self.reader.read_exact(&mut payload)?;

        // Track BOS pages
        if header_type & 0x02 != 0 {
            self.known_streams.insert(serial_no);
            return Ok(true);
        }

        // Split payload into packets using segment table lacing values.
        // A lacing value of 255 means the segment continues the current packet.
        // A lacing value < 255 terminates the current packet.
        let mut packets: Vec<Vec<u8>> = Vec::new();
        let mut current_packet = Vec::new();
        let mut offset = 0;

        // If this page has the continuation flag and we have carried-over data
        // from the previous page, prepend it.
        if header_type & 0x01 != 0 && !self.continued_packet.is_empty() {
            current_packet.append(&mut self.continued_packet);
        } else {
            self.continued_packet.clear();
        }

        for &lacing in &segment_table {
            let seg_size = lacing as usize;
            current_packet.extend_from_slice(&payload[offset..offset + seg_size]);
            offset += seg_size;

            if lacing < 255 {
                // Packet complete
                if !current_packet.is_empty() {
                    packets.push(std::mem::take(&mut current_packet));
                }
            }
            // lacing == 255: packet continues in next segment (or next page)
        }

        // If the last lacing value was 255, the packet continues on the next page
        if !current_packet.is_empty() {
            self.continued_packet = current_packet;
            self.continued_serial = serial_no;
        }

        // Handle EOS
        if header_type & 0x04 != 0 {
            self.eos_streams.insert(serial_no);
            if !self.known_streams.is_empty()
                && self.eos_streams.len() >= self.known_streams.len()
            {
                self.all_eos = true;
            }
        }

        // Filter out header packets (OpusHead, OpusTags) and enqueue data packets
        for pkt_data in packets {
            if pkt_data.len() >= 8
                && (&pkt_data[..8] == b"OpusHead" || &pkt_data[..8] == b"OpusTags")
            {
                continue;
            }
            if pkt_data.is_empty() {
                continue;
            }
            self.pending.push_back(OpusPacket {
                data: pkt_data,
                granule,
                serial_no,
            });
        }

        Ok(true)
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
        let mut ogg_data = Vec::new();
        {
            let mut writer = OpusWriter::new(&mut ogg_data, 48000, 1).unwrap();
            writer.write_frame(&[0xFC, 0x01, 0x02, 0x03], 960).unwrap();
            writer.write_frame(&[0xFC, 0x04, 0x05, 0x06], 960).unwrap();
            writer.write_frame(&[0xFC, 0x07, 0x08, 0x09], 960).unwrap();
            writer.close().unwrap();
        }

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
