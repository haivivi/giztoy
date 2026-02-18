//! Opus-in-OGG writer for multi-stream Opus audio.
//!
//! Supports writing multiple Opus streams with different serial numbers
//! into a single OGG container, with proper OpusHead/OpusTags headers
//! and granule position tracking.

use std::collections::HashMap;
use std::io::{self, Write};

/// Normal data page (no special flags). OGG spec: 0x00 = fresh packet.
/// Note: OGG continuation flag is 0x01, which is different.
const PAGE_HEADER_TYPE_FRESH: u8 = 0x00;
const PAGE_HEADER_TYPE_BOS: u8 = 0x02;
const PAGE_HEADER_TYPE_EOS: u8 = 0x04;
const DEFAULT_PRE_SKIP: u16 = 3840; // 80ms at 48kHz (RFC 7845 §5.1)
const PAGE_HEADER_SIGNATURE: &[u8] = b"OggS";
const PAGE_HEADER_SIZE: usize = 27;

/// A single Opus stream within the OGG container.
struct OpusStream {
    serial_no: i32,
    sample_rate: u32,
    channels: u16,
    granule: i64,
    page_index: u32,
    ended: bool,
    tags_written: bool,
}

/// Writes Opus frames into an OGG container.
///
/// Supports multiple streams with different serial numbers.
pub struct OpusWriter<W: Write> {
    writer: W,
    streams: HashMap<i32, OpusStream>,
    default_stream: i32,
    checksum_table: [u32; 256],
    closed: bool,
}

impl<W: Write> OpusWriter<W> {
    /// Creates a new OGG Opus writer with a default stream.
    pub fn new(writer: W, sample_rate: u32, channels: u16) -> io::Result<Self> {
        let mut ow = Self {
            writer,
            streams: HashMap::new(),
            default_stream: 0,
            checksum_table: generate_checksum_table(),
            closed: false,
        };

        let serial_no = ow.stream_begin(sample_rate, channels)?;
        ow.default_stream = serial_no;
        Ok(ow)
    }

    /// Creates a new stream and writes its BOS (OpusHead) page.
    ///
    /// Per RFC 3533, all BOS pages must appear before any non-BOS pages.
    /// The OpusTags page is deferred until the first `stream_write` call.
    /// Returns the serial number assigned to this stream.
    pub fn stream_begin(&mut self, sample_rate: u32, channels: u16) -> io::Result<i32> {
        let serial_no = self.generate_serial_no();
        let mut stream = OpusStream {
            serial_no,
            sample_rate,
            channels,
            granule: 0,
            page_index: 0,
            ended: false,
            tags_written: false,
        };

        self.write_bos_page(&mut stream)?;
        self.streams.insert(serial_no, stream);
        Ok(serial_no)
    }

    fn generate_serial_no(&self) -> i32 {
        // Simple incrementing serial for deterministic behavior
        (self.streams.len() as i32) + 1
    }

    fn write_bos_page(&mut self, stream: &mut OpusStream) -> io::Result<()> {
        let mut id_header = vec![0u8; 19];
        id_header[..8].copy_from_slice(b"OpusHead");
        id_header[8] = 1; // Version
        id_header[9] = stream.channels as u8;
        id_header[10..12].copy_from_slice(&DEFAULT_PRE_SKIP.to_le_bytes());
        id_header[12..16].copy_from_slice(&stream.sample_rate.to_le_bytes());
        id_header[16..18].copy_from_slice(&0u16.to_le_bytes()); // Output gain
        id_header[18] = 0; // Channel mapping

        let page = self.create_page(&id_header, PAGE_HEADER_TYPE_BOS, 0, stream.page_index, stream.serial_no);
        self.writer.write_all(&page)?;
        stream.page_index += 1;
        Ok(())
    }

    fn write_tags_page(&mut self, stream: &mut OpusStream) -> io::Result<()> {
        let mut comment_header = vec![0u8; 22];
        comment_header[..8].copy_from_slice(b"OpusTags");
        comment_header[8..12].copy_from_slice(&6u32.to_le_bytes()); // Vendor length
        comment_header[12..18].copy_from_slice(b"giztoy"); // Vendor
        comment_header[18..22].copy_from_slice(&0u32.to_le_bytes()); // No comments

        let page = self.create_page(&comment_header, PAGE_HEADER_TYPE_FRESH, 0, stream.page_index, stream.serial_no);
        self.writer.write_all(&page)?;
        stream.page_index += 1;
        Ok(())
    }

    /// Writes an Opus frame to the default stream.
    ///
    /// `duration_48k` is the frame duration in samples at 48kHz
    /// (e.g., 960 for 20ms, 480 for 10ms).
    pub fn write_frame(&mut self, frame: &[u8], duration_48k: i64) -> io::Result<()> {
        self.stream_write(self.default_stream, frame, duration_48k)
    }

    /// Writes an Opus frame to the specified stream.
    pub fn stream_write(&mut self, serial_no: i32, frame: &[u8], duration_48k: i64) -> io::Result<()> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::BrokenPipe, "writer closed"));
        }

        let stream = self.streams.get_mut(&serial_no)
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidInput, "invalid serial number"))?;

        if stream.ended {
            return Err(io::Error::new(io::ErrorKind::BrokenPipe, "stream ended"));
        }

        // Write OpusTags page on first data write (deferred from stream_begin
        // to ensure all BOS pages precede any non-BOS pages per RFC 3533).
        if !stream.tags_written {
            stream.tags_written = true;
            self.write_tags_page_for(serial_no)?;
            // Re-borrow after mutable self call
            let stream = self.streams.get_mut(&serial_no).unwrap();
            stream.granule += duration_48k;
            let granule = stream.granule as u64;
            let page_index = stream.page_index;
            stream.page_index += 1;

            let page = self.create_page(frame, PAGE_HEADER_TYPE_FRESH, granule, page_index, serial_no);
            return self.writer.write_all(&page);
        }

        stream.granule += duration_48k;
        let granule = stream.granule as u64;
        let page_index = stream.page_index;
        stream.page_index += 1;

        let page = self.create_page(frame, PAGE_HEADER_TYPE_FRESH, granule, page_index, serial_no);
        self.writer.write_all(&page)
    }

    /// Write tags page for a specific stream.
    fn write_tags_page_for(&mut self, serial_no: i32) -> io::Result<()> {
        let stream = self.streams.get(&serial_no).unwrap();
        let page_index = stream.page_index;

        let mut comment_header = vec![0u8; 22];
        comment_header[..8].copy_from_slice(b"OpusTags");
        comment_header[8..12].copy_from_slice(&6u32.to_le_bytes());
        comment_header[12..18].copy_from_slice(b"giztoy");
        comment_header[18..22].copy_from_slice(&0u32.to_le_bytes());

        let page = self.create_page(&comment_header, PAGE_HEADER_TYPE_FRESH, 0, page_index, serial_no);
        self.writer.write_all(&page)?;

        let stream = self.streams.get_mut(&serial_no).unwrap();
        stream.page_index += 1;
        Ok(())
    }

    /// Returns the current granule position of the default stream.
    pub fn granule(&self) -> i64 {
        self.streams.get(&self.default_stream).map(|s| s.granule).unwrap_or(0)
    }

    /// Ends the specified stream by writing an EOS page.
    pub fn stream_end(&mut self, serial_no: i32) -> io::Result<()> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::BrokenPipe, "writer closed"));
        }

        let stream = self.streams.get_mut(&serial_no)
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidInput, "invalid serial number"))?;

        if stream.ended {
            return Ok(());
        }

        let granule = stream.granule as u64;
        let page_index = stream.page_index;
        stream.ended = true;

        let page = self.create_page(&[], PAGE_HEADER_TYPE_EOS, granule, page_index, serial_no);
        self.writer.write_all(&page)
    }

    /// Closes the writer, ending all active streams.
    pub fn close(mut self) -> io::Result<()> {
        if self.closed {
            return Ok(());
        }

        let serial_nos: Vec<i32> = self.streams.keys().copied().collect();
        for serial_no in serial_nos {
            let _ = self.stream_end(serial_no);
        }
        self.closed = true;
        Ok(())
    }

    fn create_page(&self, payload: &[u8], header_type: u8, granule_pos: u64, page_index: u32, serial_no: i32) -> Vec<u8> {
        let payload_len = payload.len();
        let n_segments = if payload_len > 0 { (payload_len / 255) + 1 } else { 1 };

        let mut page = vec![0u8; PAGE_HEADER_SIZE + n_segments + payload_len];

        page[..4].copy_from_slice(PAGE_HEADER_SIGNATURE);
        page[4] = 0; // Version
        page[5] = header_type;
        page[6..14].copy_from_slice(&granule_pos.to_le_bytes());
        page[14..18].copy_from_slice(&(serial_no as u32).to_le_bytes());
        page[18..22].copy_from_slice(&page_index.to_le_bytes());
        // page[22..26] = checksum (filled later)
        page[26] = n_segments as u8;

        // Segment table
        if payload_len > 0 {
            for i in 0..n_segments - 1 {
                page[PAGE_HEADER_SIZE + i] = 255;
            }
            page[PAGE_HEADER_SIZE + n_segments - 1] = (payload_len % 255) as u8;
        } else {
            page[PAGE_HEADER_SIZE] = 0;
        }

        // Payload
        page[PAGE_HEADER_SIZE + n_segments..].copy_from_slice(payload);

        // Checksum
        let mut checksum: u32 = 0;
        for &b in &page {
            checksum = (checksum << 8) ^ self.checksum_table[((checksum >> 24) as u8 ^ b) as usize];
        }
        page[22..26].copy_from_slice(&checksum.to_le_bytes());

        page
    }
}

/// An Opus packet read from an OGG container.
#[derive(Debug, Clone)]
pub struct OpusPacket {
    /// Raw Opus frame data.
    pub data: Vec<u8>,
    /// Granule position.
    pub granule: i64,
    /// Stream serial number.
    pub serial_no: i32,
}

fn generate_checksum_table() -> [u32; 256] {
    let mut table = [0u32; 256];
    const POLY: u32 = 0x04c11db7;

    for i in 0..256 {
        let mut r = (i as u32) << 24;
        for _ in 0..8 {
            if (r & 0x80000000) != 0 {
                r = (r << 1) ^ POLY;
            } else {
                r <<= 1;
            }
        }
        table[i] = r;
    }
    table
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_opus_writer_creates_valid_ogg() {
        let mut buf = Vec::new();
        let mut writer = OpusWriter::new(&mut buf, 48000, 1).unwrap();

        // Write a fake opus frame (20ms at 48kHz = 960 samples)
        writer.write_frame(&[0xFC, 0x01, 0x02], 960).unwrap();
        writer.write_frame(&[0xFC, 0x03, 0x04], 960).unwrap();

        assert_eq!(writer.granule(), 1920);

        writer.close().unwrap();

        // Verify OGG header signature
        assert!(buf.len() > 4);
        assert_eq!(&buf[..4], b"OggS");
    }

    #[test]
    fn test_multi_stream() {
        let mut buf = Vec::new();
        let mut writer = OpusWriter::new(&mut buf, 48000, 1).unwrap();

        let stream2 = writer.stream_begin(48000, 2).unwrap();

        writer.write_frame(&[0xFC, 0x01], 960).unwrap();
        writer.stream_write(stream2, &[0xFC, 0x02], 960).unwrap();

        writer.stream_end(stream2).unwrap();
        writer.close().unwrap();

        assert!(buf.len() > 100);
    }

    #[test]
    fn test_checksum_table() {
        let table = generate_checksum_table();
        assert_eq!(table[0], 0);
        assert_ne!(table[1], 0);
    }
}
