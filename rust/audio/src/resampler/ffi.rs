//! FFI bindings to libsoxr.

use std::os::raw::{c_char, c_double, c_uint, c_void};

/// Opaque soxr handle type.
pub enum SoxrHandle {}

/// Opaque error type (const char*).
pub type SoxrError = *const c_char;

/// Quality recipe constants.
pub const SOXR_QQ: c_uint = 0; // Quick cubic interpolation
pub const SOXR_LQ: c_uint = 1; // Low quality
pub const SOXR_MQ: c_uint = 2; // Medium quality
pub const SOXR_HQ: c_uint = 4; // High quality
pub const SOXR_VHQ: c_uint = 6; // Very high quality

/// Datatype constants for io_spec.
pub const SOXR_FLOAT32_I: c_uint = 0; // Interleaved float32
pub const SOXR_FLOAT64_I: c_uint = 1; // Interleaved float64
pub const SOXR_INT32_I: c_uint = 2;   // Interleaved int32
pub const SOXR_INT16_I: c_uint = 3;   // Interleaved int16

/// I/O specification structure.
#[repr(C)]
pub struct SoxrIoSpec {
    pub itype: c_uint,
    pub otype: c_uint,
    pub scale: c_double,
    pub e: *mut c_void,
    pub flags: c_uint,
}

/// Quality specification structure.
#[repr(C)]
pub struct SoxrQualitySpec {
    pub precision: c_double,
    pub phase_response: c_double,
    pub passband_end: c_double,
    pub stopband_begin: c_double,
    pub e: *mut c_void,
    pub flags: c_uint,
}

unsafe extern "C" {
    /// Creates an I/O specification.
    pub fn soxr_io_spec(
        itype: c_uint,
        otype: c_uint,
    ) -> SoxrIoSpec;

    /// Creates a quality specification.
    pub fn soxr_quality_spec(
        recipe: c_uint,
        flags: c_uint,
    ) -> SoxrQualitySpec;

    /// Creates a new soxr resampler.
    pub fn soxr_create(
        input_rate: c_double,
        output_rate: c_double,
        num_channels: c_uint,
        error: *mut SoxrError,
        io_spec: *const SoxrIoSpec,
        quality_spec: *const SoxrQualitySpec,
        runtime_spec: *const c_void,
    ) -> *mut SoxrHandle;

    /// Processes samples through the resampler.
    pub fn soxr_process(
        handle: *mut SoxrHandle,
        input: *const c_void,
        input_len: usize,
        input_done: *mut usize,
        output: *mut c_void,
        output_len: usize,
        output_done: *mut usize,
    ) -> SoxrError;

    /// Deletes the resampler and frees resources.
    pub fn soxr_delete(handle: *mut SoxrHandle);
}

/// Safe wrapper to get error message.
pub fn error_string(err: SoxrError) -> Option<String> {
    if err.is_null() {
        None
    } else {
        unsafe {
            let c_str = std::ffi::CStr::from_ptr(err);
            Some(c_str.to_string_lossy().into_owned())
        }
    }
}
