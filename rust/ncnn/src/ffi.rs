//! Raw FFI bindings for the ncnn C API.
//!
//! These declarations match `ncnn/c_api.h`. We hand-write them instead
//! of using bindgen for simplicity and control.

use std::os::raw::{c_char, c_float, c_int, c_uchar, c_void};

/// Opaque ncnn net handle.
pub type NcnnNetT = *mut c_void;
/// Opaque ncnn extractor handle.
pub type NcnnExtractorT = *mut c_void;
/// Opaque ncnn mat handle.
pub type NcnnMatT = *mut c_void;
/// Opaque ncnn option handle.
pub type NcnnOptionT = *mut c_void;

unsafe extern "C" {
    // Version
    pub fn ncnn_version() -> *const c_char;

    // Net
    pub fn ncnn_net_create() -> NcnnNetT;
    pub fn ncnn_net_destroy(net: NcnnNetT);
    pub fn ncnn_net_load_param(net: NcnnNetT, path: *const c_char) -> c_int;
    pub fn ncnn_net_load_model(net: NcnnNetT, path: *const c_char) -> c_int;
    pub fn ncnn_net_load_param_memory(net: NcnnNetT, mem: *const c_char) -> c_int;
    pub fn ncnn_net_load_model_memory(net: NcnnNetT, mem: *const c_uchar) -> c_int;
    pub fn ncnn_net_set_option(net: NcnnNetT, opt: NcnnOptionT);

    // Extractor
    pub fn ncnn_extractor_create(net: NcnnNetT) -> NcnnExtractorT;
    pub fn ncnn_extractor_destroy(ex: NcnnExtractorT);
    pub fn ncnn_extractor_set_option(ex: NcnnExtractorT, opt: NcnnOptionT);
    pub fn ncnn_extractor_input(ex: NcnnExtractorT, name: *const c_char, mat: NcnnMatT) -> c_int;
    pub fn ncnn_extractor_extract(ex: NcnnExtractorT, name: *const c_char, mat: *mut NcnnMatT) -> c_int;

    // Mat
    pub fn ncnn_mat_create_external_2d(w: c_int, h: c_int, data: *mut c_void, allocator: *mut c_void) -> NcnnMatT;
    pub fn ncnn_mat_create_external_3d(w: c_int, h: c_int, c: c_int, data: *mut c_void, allocator: *mut c_void) -> NcnnMatT;
    pub fn ncnn_mat_destroy(mat: NcnnMatT);
    pub fn ncnn_mat_get_w(mat: NcnnMatT) -> c_int;
    pub fn ncnn_mat_get_h(mat: NcnnMatT) -> c_int;
    pub fn ncnn_mat_get_c(mat: NcnnMatT) -> c_int;
    pub fn ncnn_mat_get_data(mat: NcnnMatT) -> *const c_float;

    // Option
    pub fn ncnn_option_create() -> NcnnOptionT;
    pub fn ncnn_option_destroy(opt: NcnnOptionT);
    pub fn ncnn_option_set_use_fp16_packed(opt: NcnnOptionT, enabled: c_int);
    pub fn ncnn_option_set_use_fp16_storage(opt: NcnnOptionT, enabled: c_int);
    pub fn ncnn_option_set_use_fp16_arithmetic(opt: NcnnOptionT, enabled: c_int);
    pub fn ncnn_option_set_num_threads(opt: NcnnOptionT, n: c_int);
}
