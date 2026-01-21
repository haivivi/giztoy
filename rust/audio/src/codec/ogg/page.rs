//! Ogg page structures.

/// Ogg page header.
#[derive(Debug, Clone)]
pub struct Page {
    /// Version (always 0)
    pub version: u8,
    /// Header type flags
    pub header_type: u8,
    /// Absolute granule position
    pub granule_position: i64,
    /// Stream serial number
    pub serial: u32,
    /// Page sequence number
    pub sequence: u32,
    /// CRC checksum
    pub checksum: u32,
    /// Number of segments
    pub segments: u8,
    /// Segment table
    pub segment_table: Vec<u8>,
    /// Page body data
    pub body: Vec<u8>,
}

impl Page {
    /// Creates a new empty page.
    pub fn new() -> Self {
        Self {
            version: 0,
            header_type: 0,
            granule_position: 0,
            serial: 0,
            sequence: 0,
            checksum: 0,
            segments: 0,
            segment_table: Vec::new(),
            body: Vec::new(),
        }
    }

    /// Returns true if this is a beginning-of-stream page.
    pub fn is_bos(&self) -> bool {
        (self.header_type & 0x02) != 0
    }

    /// Returns true if this is an end-of-stream page.
    pub fn is_eos(&self) -> bool {
        (self.header_type & 0x04) != 0
    }

    /// Returns true if this is a continuation page.
    pub fn is_continuation(&self) -> bool {
        (self.header_type & 0x01) != 0
    }
}

impl Default for Page {
    fn default() -> Self {
        Self::new()
    }
}

/// Header type flags.
pub mod flags {
    /// Continuation of previous packet.
    pub const CONTINUATION: u8 = 0x01;
    /// Beginning of stream.
    pub const BOS: u8 = 0x02;
    /// End of stream.
    pub const EOS: u8 = 0x04;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_page_new() {
        let page = Page::new();
        assert_eq!(page.version, 0);
        assert_eq!(page.header_type, 0);
        assert_eq!(page.granule_position, 0);
        assert_eq!(page.serial, 0);
        assert_eq!(page.sequence, 0);
        assert_eq!(page.checksum, 0);
        assert_eq!(page.segments, 0);
        assert!(page.segment_table.is_empty());
        assert!(page.body.is_empty());
    }

    #[test]
    fn test_page_default() {
        let page = Page::default();
        assert_eq!(page.version, 0);
    }

    #[test]
    fn test_page_is_bos() {
        let mut page = Page::new();
        assert!(!page.is_bos());
        
        page.header_type = flags::BOS;
        assert!(page.is_bos());
        
        page.header_type = flags::BOS | flags::CONTINUATION;
        assert!(page.is_bos());
    }

    #[test]
    fn test_page_is_eos() {
        let mut page = Page::new();
        assert!(!page.is_eos());
        
        page.header_type = flags::EOS;
        assert!(page.is_eos());
        
        page.header_type = flags::EOS | flags::CONTINUATION;
        assert!(page.is_eos());
    }

    #[test]
    fn test_page_is_continuation() {
        let mut page = Page::new();
        assert!(!page.is_continuation());
        
        page.header_type = flags::CONTINUATION;
        assert!(page.is_continuation());
    }

    #[test]
    fn test_page_combined_flags() {
        let mut page = Page::new();
        page.header_type = flags::BOS | flags::EOS | flags::CONTINUATION;
        
        assert!(page.is_bos());
        assert!(page.is_eos());
        assert!(page.is_continuation());
    }

    #[test]
    fn test_page_with_data() {
        let mut page = Page::new();
        page.version = 0;
        page.header_type = flags::BOS;
        page.granule_position = 12345;
        page.serial = 0xDEADBEEF;
        page.sequence = 42;
        page.segments = 2;
        page.segment_table = vec![255, 100];
        page.body = vec![0; 355]; // 255 + 100
        
        assert_eq!(page.version, 0);
        assert!(page.is_bos());
        assert_eq!(page.granule_position, 12345);
        assert_eq!(page.serial, 0xDEADBEEF);
        assert_eq!(page.sequence, 42);
        assert_eq!(page.segments, 2);
        assert_eq!(page.segment_table.len(), 2);
        assert_eq!(page.body.len(), 355);
    }

    #[test]
    fn test_flags_constants() {
        assert_eq!(flags::CONTINUATION, 0x01);
        assert_eq!(flags::BOS, 0x02);
        assert_eq!(flags::EOS, 0x04);
    }

    #[test]
    fn test_page_clone() {
        let mut page = Page::new();
        page.serial = 12345;
        page.body = vec![1, 2, 3];
        
        let cloned = page.clone();
        assert_eq!(cloned.serial, page.serial);
        assert_eq!(cloned.body, page.body);
    }

    #[test]
    fn test_page_debug() {
        let page = Page::new();
        let debug_str = format!("{:?}", page);
        assert!(debug_str.contains("Page"));
    }
}
