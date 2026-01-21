//! Ogg stream state.

/// Ogg stream state for reading/writing.
#[derive(Debug)]
pub struct StreamState {
    serial: u32,
    sequence: u32,
    granule_position: i64,
}

impl StreamState {
    /// Creates a new stream state.
    pub fn new(serial: u32) -> Self {
        Self {
            serial,
            sequence: 0,
            granule_position: 0,
        }
    }

    /// Returns the serial number.
    pub fn serial(&self) -> u32 {
        self.serial
    }

    /// Returns the current sequence number.
    pub fn sequence(&self) -> u32 {
        self.sequence
    }

    /// Returns the granule position.
    pub fn granule_position(&self) -> i64 {
        self.granule_position
    }

    /// Sets the granule position.
    pub fn set_granule_position(&mut self, pos: i64) {
        self.granule_position = pos;
    }

    /// Increments the sequence number.
    pub fn increment_sequence(&mut self) {
        self.sequence += 1;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_stream_state_new() {
        let state = StreamState::new(12345);
        assert_eq!(state.serial(), 12345);
        assert_eq!(state.sequence(), 0);
        assert_eq!(state.granule_position(), 0);
    }

    #[test]
    fn test_stream_state_serial() {
        let state = StreamState::new(0xDEADBEEF);
        assert_eq!(state.serial(), 0xDEADBEEF);
    }

    #[test]
    fn test_stream_state_sequence() {
        let mut state = StreamState::new(1);
        assert_eq!(state.sequence(), 0);
        
        state.increment_sequence();
        assert_eq!(state.sequence(), 1);
        
        state.increment_sequence();
        assert_eq!(state.sequence(), 2);
    }

    #[test]
    fn test_stream_state_granule_position() {
        let mut state = StreamState::new(1);
        assert_eq!(state.granule_position(), 0);
        
        state.set_granule_position(1000);
        assert_eq!(state.granule_position(), 1000);
        
        state.set_granule_position(-1); // Granule can be -1 for unknown
        assert_eq!(state.granule_position(), -1);
    }

    #[test]
    fn test_stream_state_multiple_operations() {
        let mut state = StreamState::new(42);
        
        // Simulate writing multiple packets
        for i in 1..=10 {
            state.set_granule_position(i * 960);
            state.increment_sequence();
        }
        
        assert_eq!(state.sequence(), 10);
        assert_eq!(state.granule_position(), 9600);
    }

    #[test]
    fn test_stream_state_debug() {
        let state = StreamState::new(123);
        let debug_str = format!("{:?}", state);
        assert!(debug_str.contains("StreamState"));
    }
}
