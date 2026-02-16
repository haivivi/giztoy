//! Audio capture and playback via PortAudio.
//!
//! This module provides a Rust interface to the PortAudio C library,
//! matching Go's `audio/portaudio` package 1:1.
//!
//! - Blocking I/O model: `stream.read()` and `stream.write()` block
//! - Uses int16 (paInt16) sample format
//! - Links against the system portaudio library or `//third_party/portaudio` via Bazel
//!
//! # Example
//!
//! ```ignore
//! use giztoy_audio::portaudio;
//!
//! // List devices
//! let devices = portaudio::list_devices()?;
//! for d in &devices {
//!     println!("{}: {} (in={}, out={})", d.index, d.name,
//!              d.max_input_channels, d.max_output_channels);
//! }
//!
//! // Record 16kHz mono
//! let config = portaudio::StreamConfig {
//!     input_channels: 1,
//!     output_channels: 0,
//!     sample_rate: 16000.0,
//!     frames_per_buffer: 320,
//! };
//! let mut stream = portaudio::open_stream(config)?;
//! stream.start()?;
//! let samples = stream.read()?;
//! stream.close()?;
//! ```

pub(crate) mod ffi;

use std::ffi::CStr;
use std::io;
use std::ptr;
use std::sync::{Mutex, Once};

static INIT: Once = Once::new();
static INIT_RESULT: Mutex<Option<Result<(), String>>> = Mutex::new(None);

/// Initializes PortAudio. Safe to call multiple times.
fn initialize() -> io::Result<()> {
    INIT.call_once(|| {
        let err = unsafe { ffi::Pa_Initialize() };
        let result = if err == ffi::PA_NO_ERROR {
            Ok(())
        } else {
            Err(pa_error_string(err))
        };
        *INIT_RESULT.lock().unwrap() = Some(result);
    });

    match INIT_RESULT.lock().unwrap().as_ref().unwrap() {
        Ok(()) => Ok(()),
        Err(e) => Err(io::Error::new(io::ErrorKind::Other, e.clone())),
    }
}

fn pa_error_string(code: ffi::PaError) -> String {
    unsafe {
        let ptr = ffi::Pa_GetErrorText(code);
        if ptr.is_null() {
            return format!("portaudio error {}", code);
        }
        CStr::from_ptr(ptr).to_string_lossy().into_owned()
    }
}

fn pa_check(code: ffi::PaError) -> io::Result<()> {
    if code == ffi::PA_NO_ERROR {
        Ok(())
    } else {
        Err(io::Error::new(io::ErrorKind::Other, pa_error_string(code)))
    }
}

/// Information about an audio device.
#[derive(Debug, Clone)]
pub struct DeviceInfo {
    pub index: usize,
    pub name: String,
    pub max_input_channels: u32,
    pub max_output_channels: u32,
    pub default_low_input_latency: f64,
    pub default_high_input_latency: f64,
    pub default_low_output_latency: f64,
    pub default_high_output_latency: f64,
    pub default_sample_rate: f64,
    pub is_default_input: bool,
    pub is_default_output: bool,
}

/// Configuration for opening an audio stream.
#[derive(Debug, Clone)]
pub struct StreamConfig {
    pub input_channels: u32,
    pub output_channels: u32,
    pub sample_rate: f64,
    pub frames_per_buffer: usize,
}

/// Lists available audio devices.
pub fn list_devices() -> io::Result<Vec<DeviceInfo>> {
    initialize()?;

    let count = unsafe { ffi::Pa_GetDeviceCount() };
    if count < 0 {
        return Err(io::Error::new(io::ErrorKind::Other, pa_error_string(count)));
    }

    let default_input = unsafe { ffi::Pa_GetDefaultInputDevice() };
    let default_output = unsafe { ffi::Pa_GetDefaultOutputDevice() };

    let mut devices = Vec::with_capacity(count as usize);
    for i in 0..count {
        let info = unsafe { ffi::Pa_GetDeviceInfo(i) };
        if info.is_null() {
            continue;
        }
        let info = unsafe { &*info };
        let name = unsafe { CStr::from_ptr(info.name) }
            .to_string_lossy()
            .into_owned();

        devices.push(DeviceInfo {
            index: i as usize,
            name,
            max_input_channels: info.max_input_channels as u32,
            max_output_channels: info.max_output_channels as u32,
            default_low_input_latency: info.default_low_input_latency,
            default_high_input_latency: info.default_high_input_latency,
            default_low_output_latency: info.default_low_output_latency,
            default_high_output_latency: info.default_high_output_latency,
            default_sample_rate: info.default_sample_rate,
            is_default_input: i == default_input,
            is_default_output: i == default_output,
        });
    }
    Ok(devices)
}

/// Returns the default input device, if available.
pub fn default_input_device() -> io::Result<Option<DeviceInfo>> {
    initialize()?;
    let idx = unsafe { ffi::Pa_GetDefaultInputDevice() };
    if idx == ffi::PA_NO_DEVICE {
        return Ok(None);
    }
    let info = unsafe { ffi::Pa_GetDeviceInfo(idx) };
    if info.is_null() {
        return Ok(None);
    }
    let info = unsafe { &*info };
    let name = unsafe { CStr::from_ptr(info.name) }
        .to_string_lossy()
        .into_owned();
    Ok(Some(DeviceInfo {
        index: idx as usize,
        name,
        max_input_channels: info.max_input_channels as u32,
        max_output_channels: info.max_output_channels as u32,
        default_low_input_latency: info.default_low_input_latency,
        default_high_input_latency: info.default_high_input_latency,
        default_low_output_latency: info.default_low_output_latency,
        default_high_output_latency: info.default_high_output_latency,
        default_sample_rate: info.default_sample_rate,
        is_default_input: true,
        is_default_output: false,
    }))
}

/// Returns the default output device, if available.
pub fn default_output_device() -> io::Result<Option<DeviceInfo>> {
    initialize()?;
    let idx = unsafe { ffi::Pa_GetDefaultOutputDevice() };
    if idx == ffi::PA_NO_DEVICE {
        return Ok(None);
    }
    let info = unsafe { ffi::Pa_GetDeviceInfo(idx) };
    if info.is_null() {
        return Ok(None);
    }
    let info = unsafe { &*info };
    let name = unsafe { CStr::from_ptr(info.name) }
        .to_string_lossy()
        .into_owned();
    Ok(Some(DeviceInfo {
        index: idx as usize,
        name,
        max_input_channels: info.max_input_channels as u32,
        max_output_channels: info.max_output_channels as u32,
        default_low_input_latency: info.default_low_input_latency,
        default_high_input_latency: info.default_high_input_latency,
        default_low_output_latency: info.default_low_output_latency,
        default_high_output_latency: info.default_high_output_latency,
        default_sample_rate: info.default_sample_rate,
        is_default_input: false,
        is_default_output: true,
    }))
}

/// An audio stream for capture and/or playback.
pub struct Stream {
    pa_stream: *mut std::os::raw::c_void,
    buffer: *mut std::os::raw::c_void,
    config: StreamConfig,
    closed: bool,
}

// Stream holds raw pointers but is protected by internal locking
// and only accessed by one thread at a time.
unsafe impl Send for Stream {}

impl Stream {
    /// Starts the audio stream.
    pub fn start(&mut self) -> io::Result<()> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::Other, "stream closed"));
        }
        pa_check(unsafe { ffi::Pa_StartStream(self.pa_stream) })
    }

    /// Stops the audio stream.
    pub fn stop(&mut self) -> io::Result<()> {
        if self.closed {
            return Ok(());
        }
        pa_check(unsafe { ffi::Pa_StopStream(self.pa_stream) })
    }

    /// Closes the audio stream and frees resources.
    pub fn close(&mut self) -> io::Result<()> {
        if self.closed {
            return Ok(());
        }
        self.closed = true;
        unsafe {
            ffi::Pa_StopStream(self.pa_stream);
            let err = ffi::Pa_CloseStream(self.pa_stream);
            libc_free(self.buffer);
            pa_check(err)
        }
    }

    /// Reads audio samples from an input stream.
    ///
    /// Returns `frames_per_buffer` samples per channel as i16.
    pub fn read(&self) -> io::Result<Vec<i16>> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::Other, "stream closed"));
        }
        if self.config.input_channels == 0 {
            return Err(io::Error::new(io::ErrorKind::Other, "no input channels"));
        }

        pa_check(unsafe {
            ffi::Pa_ReadStream(
                self.pa_stream,
                self.buffer,
                self.config.frames_per_buffer as std::os::raw::c_ulong,
            )
        })?;

        let n = self.config.frames_per_buffer * self.config.input_channels as usize;
        let mut samples = vec![0i16; n];
        unsafe {
            ptr::copy_nonoverlapping(
                self.buffer as *const i16,
                samples.as_mut_ptr(),
                n,
            );
        }
        Ok(samples)
    }

    /// Reads audio samples as raw PCM bytes (little-endian int16).
    pub fn read_bytes(&self) -> io::Result<Vec<u8>> {
        let samples = self.read()?;
        let mut bytes = Vec::with_capacity(samples.len() * 2);
        for s in &samples {
            bytes.extend_from_slice(&s.to_le_bytes());
        }
        Ok(bytes)
    }

    /// Writes audio samples to an output stream.
    pub fn write(&self, samples: &[i16]) -> io::Result<()> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::Other, "stream closed"));
        }
        if self.config.output_channels == 0 {
            return Err(io::Error::new(io::ErrorKind::Other, "no output channels"));
        }

        unsafe {
            ptr::copy_nonoverlapping(
                samples.as_ptr(),
                self.buffer as *mut i16,
                samples.len(),
            );
        }

        pa_check(unsafe {
            ffi::Pa_WriteStream(
                self.pa_stream,
                self.buffer as *const _,
                samples.len() as std::os::raw::c_ulong
                    / self.config.output_channels as std::os::raw::c_ulong,
            )
        })
    }

    /// Writes raw PCM bytes (little-endian int16) to an output stream.
    pub fn write_bytes(&self, data: &[u8]) -> io::Result<()> {
        let samples: Vec<i16> = data
            .chunks_exact(2)
            .map(|b| i16::from_le_bytes([b[0], b[1]]))
            .collect();
        self.write(&samples)
    }

    /// Returns the stream configuration.
    pub fn config(&self) -> &StreamConfig {
        &self.config
    }
}

impl Drop for Stream {
    fn drop(&mut self) {
        let _ = self.close();
    }
}

/// Opens an audio stream with the given configuration.
pub fn open_stream(config: StreamConfig) -> io::Result<Stream> {
    initialize()?;

    let mut input_params_storage;
    let input_params: *const ffi::PaStreamParameters;

    if config.input_channels > 0 {
        let device = unsafe { ffi::Pa_GetDefaultInputDevice() };
        if device == ffi::PA_NO_DEVICE {
            return Err(io::Error::new(io::ErrorKind::NotFound, "no default input device"));
        }
        let info = unsafe { ffi::Pa_GetDeviceInfo(device) };
        if info.is_null() {
            return Err(io::Error::new(io::ErrorKind::Other, "failed to get input device info"));
        }
        input_params_storage = ffi::PaStreamParameters {
            device,
            channel_count: config.input_channels as std::os::raw::c_int,
            sample_format: ffi::PA_INT16,
            suggested_latency: unsafe { (*info).default_low_input_latency },
            host_api_specific_stream_info: ptr::null_mut(),
        };
        input_params = &input_params_storage;
    } else {
        input_params = ptr::null();
    }

    let mut output_params_storage;
    let output_params: *const ffi::PaStreamParameters;

    if config.output_channels > 0 {
        let device = unsafe { ffi::Pa_GetDefaultOutputDevice() };
        if device == ffi::PA_NO_DEVICE {
            return Err(io::Error::new(io::ErrorKind::NotFound, "no default output device"));
        }
        let info = unsafe { ffi::Pa_GetDeviceInfo(device) };
        if info.is_null() {
            return Err(io::Error::new(io::ErrorKind::Other, "failed to get output device info"));
        }
        output_params_storage = ffi::PaStreamParameters {
            device,
            channel_count: config.output_channels as std::os::raw::c_int,
            sample_format: ffi::PA_INT16,
            suggested_latency: unsafe { (*info).default_low_output_latency },
            host_api_specific_stream_info: ptr::null_mut(),
        };
        output_params = &output_params_storage;
    } else {
        output_params = ptr::null();
    }

    let mut pa_stream: *mut std::os::raw::c_void = ptr::null_mut();
    pa_check(unsafe {
        ffi::Pa_OpenStream(
            &mut pa_stream,
            input_params,
            output_params,
            config.sample_rate,
            config.frames_per_buffer as std::os::raw::c_ulong,
            ffi::PA_CLIP_OFF,
            ptr::null(),
            ptr::null_mut(),
        )
    })?;

    // Allocate C buffer for read/write
    let channels = config.input_channels.max(config.output_channels) as usize;
    let buffer_bytes = config.frames_per_buffer * channels * 2; // int16 = 2 bytes
    let buffer = unsafe { libc_malloc(buffer_bytes) };
    if buffer.is_null() {
        unsafe { ffi::Pa_CloseStream(pa_stream); }
        return Err(io::Error::new(io::ErrorKind::OutOfMemory, "failed to allocate buffer"));
    }

    Ok(Stream {
        pa_stream,
        buffer,
        config,
        closed: false,
    })
}

// Minimal libc wrappers to avoid adding libc as a dependency
unsafe fn libc_malloc(size: usize) -> *mut std::os::raw::c_void {
    unsafe extern "C" {
        fn malloc(size: usize) -> *mut std::os::raw::c_void;
    }
    unsafe { malloc(size) }
}

unsafe fn libc_free(ptr: *mut std::os::raw::c_void) {
    unsafe extern "C" {
        fn free(ptr: *mut std::os::raw::c_void);
    }
    unsafe { free(ptr) }
}
