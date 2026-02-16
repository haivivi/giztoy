//! Safe Rust wrappers for ncnn Net, Extractor, Mat, and NcnnOption.

use std::ffi::{CStr, CString};
use std::ptr;

use crate::error::NcnnError;
use crate::ffi;

/// Returns the ncnn library version string.
pub fn version() -> String {
    unsafe {
        let ptr = ffi::ncnn_version();
        CStr::from_ptr(ptr).to_string_lossy().into_owned()
    }
}

// ---------------------------------------------------------------------------
// NcnnOption
// ---------------------------------------------------------------------------

/// Inference options (FP16, thread count, etc.).
pub struct NcnnOption {
    pub(crate) opt: ffi::NcnnOptionT,
}

impl NcnnOption {
    /// Creates a new option with default settings.
    pub fn new() -> Result<Self, NcnnError> {
        let opt = unsafe { ffi::ncnn_option_create() };
        if opt.is_null() {
            return Err(NcnnError::Internal("option_create failed".into()));
        }
        Ok(Self { opt })
    }

    /// Enables or disables FP16 optimizations.
    /// Disable for models with intermediate values >65504 (e.g., Silero VAD).
    pub fn set_fp16(&mut self, enabled: bool) -> &mut Self {
        let v = if enabled { 1 } else { 0 };
        unsafe {
            ffi::ncnn_option_set_use_fp16_packed(self.opt, v);
            ffi::ncnn_option_set_use_fp16_storage(self.opt, v);
            ffi::ncnn_option_set_use_fp16_arithmetic(self.opt, v);
        }
        self
    }

    /// Sets the number of CPU threads for inference.
    pub fn set_num_threads(&mut self, n: i32) -> &mut Self {
        unsafe {
            ffi::ncnn_option_set_num_threads(self.opt, n);
        }
        self
    }
}

impl Drop for NcnnOption {
    fn drop(&mut self) {
        if !self.opt.is_null() {
            unsafe { ffi::ncnn_option_destroy(self.opt) };
            self.opt = ptr::null_mut();
        }
    }
}

// ---------------------------------------------------------------------------
// Net
// ---------------------------------------------------------------------------

/// Holds a loaded ncnn model.
///
/// A Net is safe for concurrent use — multiple Extractors can run in parallel
/// on the same Net. Each Extractor must be used from a single thread.
pub struct Net {
    net: ffi::NcnnNetT,
}

// ncnn Net is internally thread-safe (read-only after load).
unsafe impl Send for Net {}
unsafe impl Sync for Net {}

impl Net {
    /// Loads a model from .param and .bin files on disk.
    pub fn from_file(param_path: &str, bin_path: &str) -> Result<Self, NcnnError> {
        let net = unsafe { ffi::ncnn_net_create() };
        if net.is_null() {
            return Err(NcnnError::Internal("net_create failed".into()));
        }

        let c_param = CString::new(param_path).map_err(|e| NcnnError::Internal(e.to_string()))?;
        let ret = unsafe { ffi::ncnn_net_load_param(net, c_param.as_ptr()) };
        if ret != 0 {
            unsafe { ffi::ncnn_net_destroy(net) };
            return Err(NcnnError::Internal(format!("load_param {param_path:?}: {ret}")));
        }

        let c_bin = CString::new(bin_path).map_err(|e| NcnnError::Internal(e.to_string()))?;
        let ret = unsafe { ffi::ncnn_net_load_model(net, c_bin.as_ptr()) };
        if ret != 0 {
            unsafe { ffi::ncnn_net_destroy(net) };
            return Err(NcnnError::Internal(format!("load_model {bin_path:?}: {ret}")));
        }

        Ok(Self { net })
    }

    /// Loads a model from in-memory .param and .bin data.
    ///
    /// This is the preferred constructor when the model is embedded in the
    /// binary via `include_bytes!`.
    pub fn from_memory(param_data: &[u8], bin_data: &[u8], opt: Option<&NcnnOption>) -> Result<Self, NcnnError> {
        if param_data.is_empty() {
            return Err(NcnnError::EmptyData);
        }
        if bin_data.is_empty() {
            return Err(NcnnError::EmptyData);
        }

        let net = unsafe { ffi::ncnn_net_create() };
        if net.is_null() {
            return Err(NcnnError::Internal("net_create failed".into()));
        }

        // Apply option BEFORE loading.
        if let Some(o) = opt {
            unsafe { ffi::ncnn_net_set_option(net, o.opt) };
        }

        // ncnn_net_load_param_memory expects a null-terminated C string.
        let c_param = CString::new(param_data).map_err(|e| NcnnError::Internal(e.to_string()))?;
        let ret = unsafe { ffi::ncnn_net_load_param_memory(net, c_param.as_ptr()) };
        if ret != 0 {
            unsafe { ffi::ncnn_net_destroy(net) };
            return Err(NcnnError::Internal(format!("load_param_memory: {ret}")));
        }

        let ret = unsafe { ffi::ncnn_net_load_model_memory(net, bin_data.as_ptr()) };
        if ret < 0 {
            unsafe { ffi::ncnn_net_destroy(net) };
            return Err(NcnnError::Internal(format!("load_model_memory: {ret}")));
        }

        Ok(Self { net })
    }

    /// Sets an option on this Net. Must be called before creating Extractors.
    pub fn set_option(&self, opt: &NcnnOption) {
        unsafe { ffi::ncnn_net_set_option(self.net, opt.opt) };
    }

    /// Creates a new inference session (Extractor) for this Net.
    pub fn extractor(&self) -> Result<Extractor, NcnnError> {
        let ex = unsafe { ffi::ncnn_extractor_create(self.net) };
        if ex.is_null() {
            return Err(NcnnError::Internal("extractor_create failed".into()));
        }
        Ok(Extractor { ex })
    }
}

impl Drop for Net {
    fn drop(&mut self) {
        if !self.net.is_null() {
            unsafe { ffi::ncnn_net_destroy(self.net) };
            self.net = ptr::null_mut();
        }
    }
}

// ---------------------------------------------------------------------------
// Extractor
// ---------------------------------------------------------------------------

/// Runs inference on a loaded Net.
///
/// An Extractor must be used from a single thread.
pub struct Extractor {
    ex: ffi::NcnnExtractorT,
}

impl Extractor {
    /// Feeds a Mat as input to the named blob.
    pub fn set_input(&mut self, name: &str, mat: &Mat) -> Result<(), NcnnError> {
        let c_name = CString::new(name).map_err(|e| NcnnError::Internal(e.to_string()))?;
        let ret = unsafe { ffi::ncnn_extractor_input(self.ex, c_name.as_ptr(), mat.mat) };
        if ret != 0 {
            return Err(NcnnError::Internal(format!("extractor_input {name:?}: {ret}")));
        }
        Ok(())
    }

    /// Runs inference and returns the output Mat for the named blob.
    pub fn extract(&mut self, name: &str) -> Result<Mat, NcnnError> {
        let c_name = CString::new(name).map_err(|e| NcnnError::Internal(e.to_string()))?;
        let mut mat: ffi::NcnnMatT = ptr::null_mut();
        let ret = unsafe { ffi::ncnn_extractor_extract(self.ex, c_name.as_ptr(), &mut mat) };
        if ret != 0 {
            return Err(NcnnError::Internal(format!("extractor_extract {name:?}: {ret}")));
        }
        Ok(Mat { mat, _pinned: None })
    }

    /// Sets an option on this Extractor.
    pub fn set_option(&mut self, opt: &NcnnOption) {
        unsafe { ffi::ncnn_extractor_set_option(self.ex, opt.opt) };
    }
}

impl Drop for Extractor {
    fn drop(&mut self) {
        if !self.ex.is_null() {
            unsafe { ffi::ncnn_extractor_destroy(self.ex) };
            self.ex = ptr::null_mut();
        }
    }
}

// ---------------------------------------------------------------------------
// Mat
// ---------------------------------------------------------------------------

/// N-dimensional tensor for input/output data.
pub struct Mat {
    mat: ffi::NcnnMatT,
    // Prevents the backing data from being dropped while this Mat is alive.
    _pinned: Option<Vec<f32>>,
}

impl Mat {
    /// Creates a 2D Mat (h rows x w cols) backed by the provided data.
    ///
    /// The data is owned by this Mat (copied internally to ensure safety).
    pub fn new_2d(w: i32, h: i32, data: &[f32]) -> Result<Self, NcnnError> {
        if data.is_empty() {
            return Err(NcnnError::EmptyData);
        }
        let required = (w * h) as usize;
        if data.len() < required {
            return Err(NcnnError::Internal(format!(
                "data too short: got {}, need {required} (w={w}, h={h})",
                data.len()
            )));
        }

        let mut owned = data[..required].to_vec();
        let mat = unsafe {
            ffi::ncnn_mat_create_external_2d(
                w,
                h,
                owned.as_mut_ptr() as *mut _,
                ptr::null_mut(),
            )
        };
        if mat.is_null() {
            return Err(NcnnError::Internal("mat_create_external_2d failed".into()));
        }
        Ok(Self {
            mat,
            _pinned: Some(owned),
        })
    }

    /// Creates a 3D Mat (c channels x h rows x w cols) backed by the provided data.
    pub fn new_3d(w: i32, h: i32, c: i32, data: &[f32]) -> Result<Self, NcnnError> {
        if data.is_empty() {
            return Err(NcnnError::EmptyData);
        }
        let required = (w * h * c) as usize;
        if data.len() < required {
            return Err(NcnnError::Internal(format!(
                "data too short: got {}, need {required} (w={w}, h={h}, c={c})",
                data.len()
            )));
        }

        let mut owned = data[..required].to_vec();
        let mat = unsafe {
            ffi::ncnn_mat_create_external_3d(
                w,
                h,
                c,
                owned.as_mut_ptr() as *mut _,
                ptr::null_mut(),
            )
        };
        if mat.is_null() {
            return Err(NcnnError::Internal("mat_create_external_3d failed".into()));
        }
        Ok(Self {
            mat,
            _pinned: Some(owned),
        })
    }

    /// Returns the width (first dimension).
    pub fn w(&self) -> i32 {
        unsafe { ffi::ncnn_mat_get_w(self.mat) }
    }

    /// Returns the height (second dimension).
    pub fn h(&self) -> i32 {
        unsafe { ffi::ncnn_mat_get_h(self.mat) }
    }

    /// Returns the number of channels (third dimension).
    pub fn c(&self) -> i32 {
        unsafe { ffi::ncnn_mat_get_c(self.mat) }
    }

    /// Copies the Mat data into a new f32 slice.
    pub fn float_data(&self) -> Option<Vec<f32>> {
        let ptr = unsafe { ffi::ncnn_mat_get_data(self.mat) };
        if ptr.is_null() {
            return None;
        }
        let mut n = (self.w() * self.h() * self.c()) as usize;
        if n == 0 {
            n = self.w() as usize;
        }
        if n == 0 {
            return None;
        }
        let mut out = vec![0.0f32; n];
        unsafe {
            ptr::copy_nonoverlapping(ptr, out.as_mut_ptr(), n);
        }
        Some(out)
    }
}

impl Drop for Mat {
    fn drop(&mut self) {
        if !self.mat.is_null() {
            unsafe { ffi::ncnn_mat_destroy(self.mat) };
            self.mat = ptr::null_mut();
        }
    }
}
