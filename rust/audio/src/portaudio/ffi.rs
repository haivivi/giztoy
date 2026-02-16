//! Raw FFI bindings for PortAudio.
//!
//! These bindings match the PortAudio C API used by the Go package
//! (`go/pkg/audio/portaudio/portaudio.go`). The Bazel build links
//! against `//third_party/portaudio`.

use std::os::raw::{c_char, c_double, c_int, c_ulong, c_void};

pub type PaError = c_int;
pub type PaDeviceIndex = c_int;
pub type PaStreamFlags = c_ulong;

pub const PA_NO_ERROR: PaError = 0;
pub const PA_NO_DEVICE: PaDeviceIndex = -1;

pub const PA_INT16: c_ulong = 0x00000008;
pub const PA_CLIP_OFF: PaStreamFlags = 0x00000001;

#[repr(C)]
pub struct PaDeviceInfo {
    pub struct_version: c_int,
    pub name: *const c_char,
    pub host_api: c_int,
    pub max_input_channels: c_int,
    pub max_output_channels: c_int,
    pub default_low_input_latency: c_double,
    pub default_low_output_latency: c_double,
    pub default_high_input_latency: c_double,
    pub default_high_output_latency: c_double,
    pub default_sample_rate: c_double,
}

#[repr(C)]
pub struct PaStreamParameters {
    pub device: PaDeviceIndex,
    pub channel_count: c_int,
    pub sample_format: c_ulong,
    pub suggested_latency: c_double,
    pub host_api_specific_stream_info: *mut c_void,
}

unsafe extern "C" {
    pub fn Pa_Initialize() -> PaError;
    pub fn Pa_Terminate() -> PaError;
    pub fn Pa_GetErrorText(error_code: PaError) -> *const c_char;

    pub fn Pa_GetDeviceCount() -> PaDeviceIndex;
    pub fn Pa_GetDefaultInputDevice() -> PaDeviceIndex;
    pub fn Pa_GetDefaultOutputDevice() -> PaDeviceIndex;
    pub fn Pa_GetDeviceInfo(device: PaDeviceIndex) -> *const PaDeviceInfo;

    pub fn Pa_OpenStream(
        stream: *mut *mut c_void,
        input_parameters: *const PaStreamParameters,
        output_parameters: *const PaStreamParameters,
        sample_rate: c_double,
        frames_per_buffer: c_ulong,
        stream_flags: PaStreamFlags,
        stream_callback: *const c_void, // NULL for blocking I/O
        user_data: *mut c_void,
    ) -> PaError;

    pub fn Pa_StartStream(stream: *mut c_void) -> PaError;
    pub fn Pa_StopStream(stream: *mut c_void) -> PaError;
    pub fn Pa_CloseStream(stream: *mut c_void) -> PaError;

    pub fn Pa_ReadStream(
        stream: *mut c_void,
        buffer: *mut c_void,
        frames: c_ulong,
    ) -> PaError;

    pub fn Pa_WriteStream(
        stream: *mut c_void,
        buffer: *const c_void,
        frames: c_ulong,
    ) -> PaError;
}
