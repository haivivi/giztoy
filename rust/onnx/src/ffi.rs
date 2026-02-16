//! Raw FFI bindings for the ONNX Runtime C API.
//!
//! These declarations match `onnxruntime_c_api.h`. We hand-write them
//! for the subset we need, avoiding bindgen complexity.

use std::os::raw::{c_char, c_float, c_void};

// Opaque types — the ORT C API uses opaque pointers.
pub type OrtApi = c_void;
pub type OrtEnv = c_void;
pub type OrtSession = c_void;
pub type OrtSessionOptions = c_void;
pub type OrtMemoryInfo = c_void;
pub type OrtValue = c_void;
pub type OrtStatus = c_void;

// C shim functions (defined in ort_shim.c).
// The ORT C API uses a function pointer table. Calling through function
// pointers from Rust FFI is cumbersome, so we provide thin C wrappers
// (same approach as Go's CGo helpers).
unsafe extern "C" {
    // --- Shim functions (defined in ort_shim.c) ---
    pub fn ort_api() -> *const OrtApi;
    pub fn ort_create_env(api: *const OrtApi, name: *const c_char, out: *mut *mut OrtEnv) -> *mut OrtStatus;
    pub fn ort_create_session_options(api: *const OrtApi, out: *mut *mut OrtSessionOptions) -> *mut OrtStatus;
    pub fn ort_create_session_from_memory(
        api: *const OrtApi,
        env: *mut OrtEnv,
        model_data: *const c_void,
        model_data_len: usize,
        opts: *mut OrtSessionOptions,
        out: *mut *mut OrtSession,
    ) -> *mut OrtStatus;
    pub fn ort_create_cpu_memory_info(api: *const OrtApi, out: *mut *mut OrtMemoryInfo) -> *mut OrtStatus;
    pub fn ort_create_tensor_float(
        api: *const OrtApi,
        info: *mut OrtMemoryInfo,
        data: *mut c_float,
        data_len: usize,
        shape: *const i64,
        shape_len: usize,
        out: *mut *mut OrtValue,
    ) -> *mut OrtStatus;
    pub fn ort_run(
        api: *const OrtApi,
        session: *mut OrtSession,
        input_names: *const *const c_char,
        inputs: *const *const OrtValue,
        num_inputs: usize,
        output_names: *const *const c_char,
        num_outputs: usize,
        outputs: *mut *mut OrtValue,
    ) -> *mut OrtStatus;
    pub fn ort_get_tensor_float_data(api: *const OrtApi, value: *mut OrtValue, out: *mut *mut c_float) -> *mut OrtStatus;
    pub fn ort_get_tensor_ndim(api: *const OrtApi, value: *mut OrtValue, ndim: *mut usize) -> *mut OrtStatus;
    pub fn ort_get_tensor_shape(api: *const OrtApi, value: *mut OrtValue, shape: *mut i64, shape_len: usize) -> *mut OrtStatus;
    pub fn ort_error_message(api: *const OrtApi, status: *mut OrtStatus) -> *const c_char;
    pub fn ort_release_status(api: *const OrtApi, status: *mut OrtStatus);
    pub fn ort_release_env(api: *const OrtApi, env: *mut OrtEnv);
    pub fn ort_release_session(api: *const OrtApi, s: *mut OrtSession);
    pub fn ort_release_session_options(api: *const OrtApi, o: *mut OrtSessionOptions);
    pub fn ort_release_memory_info(api: *const OrtApi, i: *mut OrtMemoryInfo);
    pub fn ort_release_value(api: *const OrtApi, v: *mut OrtValue);
}
