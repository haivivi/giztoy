//! FFI bindings to libopus.

use std::os::raw::{c_char, c_int, c_uchar};

/// Opaque encoder state.
pub enum OpusEncoder {}

/// Opaque decoder state.
pub enum OpusDecoder {}

/// opus_int32 type (from opus_types.h)
pub type OpusInt32 = i32;

/// opus_int16 type (from opus_types.h)
pub type OpusInt16 = i16;

// Return codes
pub const OPUS_OK: c_int = 0;

// Application types
pub const OPUS_APPLICATION_VOIP: c_int = 2048;
pub const OPUS_APPLICATION_AUDIO: c_int = 2049;
pub const OPUS_APPLICATION_RESTRICTED_LOWDELAY: c_int = 2051;

// CTL macros (request codes)
pub const OPUS_SET_BITRATE_REQUEST: c_int = 4002;
pub const OPUS_SET_COMPLEXITY_REQUEST: c_int = 4010;

unsafe extern "C" {
    // Error handling
    pub fn opus_strerror(error: c_int) -> *const c_char;

    // Encoder
    pub fn opus_encoder_create(
        fs: OpusInt32,
        channels: c_int,
        application: c_int,
        error: *mut c_int,
    ) -> *mut OpusEncoder;

    pub fn opus_encoder_destroy(enc: *mut OpusEncoder);

    pub fn opus_encode(
        enc: *mut OpusEncoder,
        pcm: *const OpusInt16,
        frame_size: c_int,
        data: *mut c_uchar,
        max_data_bytes: OpusInt32,
    ) -> OpusInt32;

    pub fn opus_encoder_ctl(enc: *mut OpusEncoder, request: c_int, ...) -> c_int;

    // Decoder
    pub fn opus_decoder_create(
        fs: OpusInt32,
        channels: c_int,
        error: *mut c_int,
    ) -> *mut OpusDecoder;

    pub fn opus_decoder_destroy(dec: *mut OpusDecoder);

    pub fn opus_decode(
        dec: *mut OpusDecoder,
        data: *const c_uchar,
        len: OpusInt32,
        pcm: *mut OpusInt16,
        frame_size: c_int,
        decode_fec: c_int,
    ) -> c_int;
}

/// Gets an error message for an opus error code.
pub fn error_string(error: c_int) -> String {
    unsafe {
        let c_str = opus_strerror(error);
        if c_str.is_null() {
            return format!("opus error {}", error);
        }
        std::ffi::CStr::from_ptr(c_str)
            .to_string_lossy()
            .into_owned()
    }
}
