//! FFI bindings for LAME and minimp3.

use std::os::raw::{c_int, c_uchar, c_float, c_short};

// ============ LAME Encoder FFI ============

/// Opaque LAME global flags.
pub enum LameGlobalFlags {}

// LAME mode constants
pub const MONO: c_int = 3;
pub const JOINT_STEREO: c_int = 1;

// VBR modes
pub const VBR_OFF: c_int = 0;
pub const VBR_DEFAULT: c_int = 4;

unsafe extern "C" {
    pub fn lame_init() -> *mut LameGlobalFlags;
    pub fn lame_close(gf: *mut LameGlobalFlags) -> c_int;

    pub fn lame_set_in_samplerate(gf: *mut LameGlobalFlags, rate: c_int) -> c_int;
    pub fn lame_set_num_channels(gf: *mut LameGlobalFlags, channels: c_int) -> c_int;
    pub fn lame_set_mode(gf: *mut LameGlobalFlags, mode: c_int) -> c_int;
    pub fn lame_set_VBR(gf: *mut LameGlobalFlags, vbr_mode: c_int) -> c_int;
    pub fn lame_set_VBR_quality(gf: *mut LameGlobalFlags, quality: c_float) -> c_int;
    pub fn lame_set_brate(gf: *mut LameGlobalFlags, brate: c_int) -> c_int;

    pub fn lame_init_params(gf: *mut LameGlobalFlags) -> c_int;

    pub fn lame_encode_buffer(
        gf: *mut LameGlobalFlags,
        pcm_l: *const c_short,
        pcm_r: *const c_short,
        nsamples: c_int,
        mp3buf: *mut c_uchar,
        mp3buf_size: c_int,
    ) -> c_int;

    pub fn lame_encode_buffer_interleaved(
        gf: *mut LameGlobalFlags,
        pcm: *const c_short,
        nsamples: c_int,
        mp3buf: *mut c_uchar,
        mp3buf_size: c_int,
    ) -> c_int;

    pub fn lame_encode_flush(
        gf: *mut LameGlobalFlags,
        mp3buf: *mut c_uchar,
        mp3buf_size: c_int,
    ) -> c_int;
}

// ============ minimp3 Decoder FFI ============

/// Maximum samples per frame (stereo).
pub const MINIMP3_MAX_SAMPLES_PER_FRAME: usize = 1152 * 2;

/// MP3 decoder state.
/// Size matches mp3dec_t from minimp3.h:
///   float mdct_overlap[2][9*32]  = 2304 bytes
///   float qmf_state[15*2*32]     = 3840 bytes
///   int reserv, free_format_bytes = 8 bytes
///   unsigned char header[4]      = 4 bytes
///   unsigned char reserv_buf[511] = 511 bytes
///   Total ≈ 6667 bytes, rounded up to 6672 for alignment
#[repr(C)]
pub struct Mp3Dec {
    _data: [u8; 6672], // Internal state matching minimp3's mp3dec_t
}

/// Frame info returned by decoder.
/// Matches mp3dec_frame_info_t from minimp3.h
#[repr(C)]
pub struct Mp3FrameInfo {
    pub frame_bytes: c_int,
    pub frame_offset: c_int,  // Added: offset of first frame in buffer
    pub channels: c_int,
    pub hz: c_int,
    pub layer: c_int,
    pub bitrate_kbps: c_int,
}

unsafe extern "C" {
    pub fn mp3dec_init(dec: *mut Mp3Dec);

    pub fn mp3dec_decode_frame(
        dec: *mut Mp3Dec,
        mp3: *const c_uchar,
        mp3_bytes: c_int,
        pcm: *mut c_short,
        info: *mut Mp3FrameInfo,
    ) -> c_int;
}

impl Mp3Dec {
    /// Creates and initializes a new decoder.
    pub fn new() -> Self {
        let mut dec = Self { _data: [0; 6672] };
        unsafe { mp3dec_init(&mut dec) };
        dec
    }
}

impl Default for Mp3Dec {
    fn default() -> Self {
        Self::new()
    }
}
