//! Audio capture and playback interface.
//!
//! This module provides a platform-agnostic API for audio I/O, matching
//! Go's `audio/portaudio` package. The Rust equivalent uses the `cpal`
//! crate as the backend.
//!
//! # Example
//!
//! ```ignore
//! use giztoy_audio::portaudio::{DeviceInfo, Stream, StreamConfig};
//!
//! let devices = list_devices()?;
//! for d in &devices {
//!     println!("{}: {} (in={}, out={})", d.index, d.name,
//!              d.max_input_channels, d.max_output_channels);
//! }
//!
//! let config = StreamConfig {
//!     input_channels: 1,
//!     output_channels: 0,
//!     sample_rate: 16000.0,
//!     frames_per_buffer: 320,
//! };
//! let mut stream = open_stream(config)?;
//! stream.start()?;
//!
//! let samples = stream.read()?;
//! ```

use std::io;

/// Information about an audio device.
#[derive(Debug, Clone)]
pub struct DeviceInfo {
    /// Device index.
    pub index: usize,
    /// Device name.
    pub name: String,
    /// Maximum input channels.
    pub max_input_channels: u32,
    /// Maximum output channels.
    pub max_output_channels: u32,
    /// Default sample rate.
    pub default_sample_rate: f64,
    /// Whether this is the default input device.
    pub is_default_input: bool,
    /// Whether this is the default output device.
    pub is_default_output: bool,
}

/// Configuration for opening an audio stream.
#[derive(Debug, Clone)]
pub struct StreamConfig {
    /// Number of input channels (0 = no input).
    pub input_channels: u32,
    /// Number of output channels (0 = no output).
    pub output_channels: u32,
    /// Sample rate in Hz.
    pub sample_rate: f64,
    /// Frames per buffer (determines latency).
    pub frames_per_buffer: usize,
}

/// An audio stream for capture and/or playback.
///
/// Currently a placeholder. Implement with `cpal` backend by enabling
/// the `portaudio` feature flag on this crate.
pub struct Stream {
    config: StreamConfig,
    started: bool,
    closed: bool,
}

impl Stream {
    /// Starts the audio stream.
    pub fn start(&mut self) -> io::Result<()> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::Other, "stream closed"));
        }
        self.started = true;
        Ok(())
    }

    /// Stops the audio stream.
    pub fn stop(&mut self) -> io::Result<()> {
        self.started = false;
        Ok(())
    }

    /// Closes the audio stream.
    pub fn close(&mut self) -> io::Result<()> {
        self.started = false;
        self.closed = true;
        Ok(())
    }

    /// Reads audio samples from an input stream.
    ///
    /// Returns a vector of i16 samples with length = frames_per_buffer * channels.
    pub fn read(&self) -> io::Result<Vec<i16>> {
        if !self.started {
            return Err(io::Error::new(io::ErrorKind::Other, "stream not started"));
        }
        if self.config.input_channels == 0 {
            return Err(io::Error::new(io::ErrorKind::Other, "no input channels"));
        }
        // Placeholder: return silence
        let n = self.config.frames_per_buffer * self.config.input_channels as usize;
        Ok(vec![0i16; n])
    }

    /// Writes audio samples to an output stream.
    pub fn write(&self, _samples: &[i16]) -> io::Result<()> {
        if !self.started {
            return Err(io::Error::new(io::ErrorKind::Other, "stream not started"));
        }
        if self.config.output_channels == 0 {
            return Err(io::Error::new(io::ErrorKind::Other, "no output channels"));
        }
        Ok(())
    }

    /// Returns the stream configuration.
    pub fn config(&self) -> &StreamConfig {
        &self.config
    }
}

/// Lists available audio devices.
///
/// Returns an empty list when no audio backend is available.
pub fn list_devices() -> io::Result<Vec<DeviceInfo>> {
    // Placeholder: no devices available without cpal backend
    Ok(Vec::new())
}

/// Returns the default input device, if available.
pub fn default_input_device() -> io::Result<Option<DeviceInfo>> {
    Ok(None)
}

/// Returns the default output device, if available.
pub fn default_output_device() -> io::Result<Option<DeviceInfo>> {
    Ok(None)
}

/// Opens an audio stream with the given configuration.
///
/// This is a placeholder implementation. A real implementation requires
/// the `cpal` crate for cross-platform audio I/O.
pub fn open_stream(config: StreamConfig) -> io::Result<Stream> {
    Ok(Stream {
        config,
        started: false,
        closed: false,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_list_devices() {
        let devices = list_devices().unwrap();
        // Placeholder returns empty list
        assert!(devices.is_empty());
    }

    #[test]
    fn test_stream_lifecycle() {
        let config = StreamConfig {
            input_channels: 1,
            output_channels: 0,
            sample_rate: 16000.0,
            frames_per_buffer: 320,
        };

        let mut stream = open_stream(config).unwrap();
        stream.start().unwrap();

        let samples = stream.read().unwrap();
        assert_eq!(samples.len(), 320);

        stream.stop().unwrap();
        stream.close().unwrap();

        // Double close should be ok
        stream.close().unwrap();
    }

    #[test]
    fn test_stream_not_started() {
        let config = StreamConfig {
            input_channels: 1,
            output_channels: 0,
            sample_rate: 16000.0,
            frames_per_buffer: 320,
        };

        let stream = open_stream(config).unwrap();
        assert!(stream.read().is_err());
    }
}
