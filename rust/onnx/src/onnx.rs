//! Safe Rust wrappers for ONNX Runtime Env, Session, and Tensor.

use std::ffi::CString;
use std::ptr;

use crate::error::OnnxError;
use crate::ffi;

/// Gets the ORT API pointer (cached after first call).
fn api() -> *const ffi::OrtApi {
    unsafe { ffi::ort_api() }
}

/// Converts an OrtStatus to a Rust Result.
fn check_status(status: *mut ffi::OrtStatus) -> Result<(), OnnxError> {
    if status.is_null() {
        return Ok(());
    }
    let msg = unsafe {
        let ptr = ffi::ort_error_message(api(), status);
        let s = std::ffi::CStr::from_ptr(ptr).to_string_lossy().into_owned();
        ffi::ort_release_status(api(), status);
        s
    };
    Err(OnnxError::Runtime(msg))
}

// ---------------------------------------------------------------------------
// Env
// ---------------------------------------------------------------------------

/// ONNX Runtime environment. Create one per process.
pub struct Env {
    env: *mut ffi::OrtEnv,
}

unsafe impl Send for Env {}
unsafe impl Sync for Env {}

impl Env {
    /// Creates a new ONNX Runtime environment.
    pub fn new(name: &str) -> Result<Self, OnnxError> {
        let c_name = CString::new(name).map_err(|e| OnnxError::Runtime(e.to_string()))?;
        let mut env: *mut ffi::OrtEnv = ptr::null_mut();
        check_status(unsafe { ffi::ort_create_env(api(), c_name.as_ptr(), &mut env) })?;
        Ok(Self { env })
    }

    /// Creates a session from in-memory ONNX model data.
    pub fn new_session(&self, model_data: &[u8]) -> Result<Session, OnnxError> {
        if model_data.is_empty() {
            return Err(OnnxError::EmptyData);
        }

        let mut opts: *mut ffi::OrtSessionOptions = ptr::null_mut();
        check_status(unsafe { ffi::ort_create_session_options(api(), &mut opts) })?;

        let mut session: *mut ffi::OrtSession = ptr::null_mut();
        let status = unsafe {
            ffi::ort_create_session_from_memory(
                api(),
                self.env,
                model_data.as_ptr() as *const _,
                model_data.len(),
                opts,
                &mut session,
            )
        };
        unsafe { ffi::ort_release_session_options(api(), opts) };
        check_status(status)?;

        Ok(Session { session })
    }
}

impl Drop for Env {
    fn drop(&mut self) {
        if !self.env.is_null() {
            unsafe { ffi::ort_release_env(api(), self.env) };
            self.env = ptr::null_mut();
        }
    }
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

/// Holds a loaded ONNX model.
pub struct Session {
    session: *mut ffi::OrtSession,
}

unsafe impl Send for Session {}
unsafe impl Sync for Session {}

impl Session {
    /// Runs inference with the given inputs and output names.
    pub fn run(
        &self,
        input_names: &[&str],
        inputs: &[&Tensor],
        output_names: &[&str],
    ) -> Result<Vec<Tensor>, OnnxError> {
        if input_names.len() != inputs.len() {
            return Err(OnnxError::Runtime(format!(
                "input names/tensors length mismatch: {} vs {}",
                input_names.len(),
                inputs.len()
            )));
        }

        // Prepare C input names.
        let c_input_names: Vec<CString> = input_names
            .iter()
            .map(|n| CString::new(*n).unwrap())
            .collect();
        let c_input_ptrs: Vec<*const i8> = c_input_names.iter().map(|s| s.as_ptr()).collect();

        // Prepare C input values.
        let c_inputs: Vec<*const ffi::OrtValue> = inputs.iter().map(|t| t.value as *const _).collect();

        // Prepare C output names.
        let c_output_names: Vec<CString> = output_names
            .iter()
            .map(|n| CString::new(*n).unwrap())
            .collect();
        let c_output_ptrs: Vec<*const i8> = c_output_names.iter().map(|s| s.as_ptr()).collect();

        // Allocate output values.
        let mut c_outputs: Vec<*mut ffi::OrtValue> = vec![ptr::null_mut(); output_names.len()];

        check_status(unsafe {
            ffi::ort_run(
                api(),
                self.session,
                c_input_ptrs.as_ptr(),
                c_inputs.as_ptr(),
                inputs.len(),
                c_output_ptrs.as_ptr(),
                output_names.len(),
                c_outputs.as_mut_ptr(),
            )
        })?;

        let outputs: Vec<Tensor> = c_outputs
            .into_iter()
            .map(|v| Tensor {
                value: v,
                _pinned: None,
            })
            .collect();

        Ok(outputs)
    }
}

impl Drop for Session {
    fn drop(&mut self) {
        if !self.session.is_null() {
            unsafe { ffi::ort_release_session(api(), self.session) };
            self.session = ptr::null_mut();
        }
    }
}

// ---------------------------------------------------------------------------
// Tensor
// ---------------------------------------------------------------------------

/// N-dimensional float32 tensor.
pub struct Tensor {
    value: *mut ffi::OrtValue,
    _pinned: Option<Vec<f32>>,
}

impl Tensor {
    /// Creates a float32 tensor with the given shape and data.
    pub fn new(shape: &[i64], data: &[f32]) -> Result<Self, OnnxError> {
        if data.is_empty() {
            return Err(OnnxError::EmptyData);
        }

        let total: i64 = shape.iter().product();
        if (data.len() as i64) < total {
            return Err(OnnxError::Runtime(format!(
                "tensor data too short: got {}, need {total}",
                data.len()
            )));
        }

        let mut mem_info: *mut ffi::OrtMemoryInfo = ptr::null_mut();
        check_status(unsafe { ffi::ort_create_cpu_memory_info(api(), &mut mem_info) })?;

        let mut owned = data[..total as usize].to_vec();
        let mut value: *mut ffi::OrtValue = ptr::null_mut();

        let status = unsafe {
            ffi::ort_create_tensor_float(
                api(),
                mem_info,
                owned.as_mut_ptr(),
                owned.len(),
                shape.as_ptr(),
                shape.len(),
                &mut value,
            )
        };
        unsafe { ffi::ort_release_memory_info(api(), mem_info) };
        check_status(status)?;

        Ok(Self {
            value,
            _pinned: Some(owned),
        })
    }

    /// Copies the tensor data into a new f32 slice.
    pub fn float_data(&self) -> Result<Vec<f32>, OnnxError> {
        let mut ptr: *mut f32 = ptr::null_mut();
        check_status(unsafe { ffi::ort_get_tensor_float_data(api(), self.value, &mut ptr) })?;

        let shape = self.shape()?;
        let total: usize = shape.iter().map(|&d| d as usize).product();
        if total == 0 {
            return Ok(Vec::new());
        }

        let mut out = vec![0.0f32; total];
        unsafe {
            ptr::copy_nonoverlapping(ptr, out.as_mut_ptr(), total);
        }
        Ok(out)
    }

    /// Returns the tensor dimensions.
    pub fn shape(&self) -> Result<Vec<i64>, OnnxError> {
        let mut ndim: usize = 0;
        check_status(unsafe { ffi::ort_get_tensor_ndim(api(), self.value, &mut ndim) })?;

        if ndim == 0 {
            return Ok(Vec::new());
        }

        let mut shape = vec![0i64; ndim];
        check_status(unsafe { ffi::ort_get_tensor_shape(api(), self.value, shape.as_mut_ptr(), ndim) })?;
        Ok(shape)
    }
}

impl Drop for Tensor {
    fn drop(&mut self) {
        if !self.value.is_null() {
            unsafe { ffi::ort_release_value(api(), self.value) };
            self.value = ptr::null_mut();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use once_cell::sync::Lazy;
    use std::sync::Mutex;

    // ONNX Runtime only allows one global Env per process.
    // Tests run in parallel, so we share a single Env.
    static TEST_ENV: Lazy<Mutex<Env>> = Lazy::new(|| {
        Mutex::new(Env::new("test").unwrap())
    });

    #[test]
    fn env_create() {
        let _env = TEST_ENV.lock().unwrap();
    }

    #[test]
    fn tensor_create_and_read() {
        let data: Vec<f32> = (0..12).map(|i| i as f32).collect();
        let tensor = Tensor::new(&[3, 4], &data).unwrap();
        let shape = tensor.shape().unwrap();
        assert_eq!(shape, vec![3, 4]);
        let out = tensor.float_data().unwrap();
        assert_eq!(out.len(), 12);
        assert_eq!(out, data);
    }

    #[test]
    fn tensor_empty_error() {
        assert!(Tensor::new(&[1], &[]).is_err());
    }

    #[test]
    fn speaker_model_inference() {
        crate::model_embed::register_embedded_models();

        let env = TEST_ENV.lock().unwrap();
        let model_data = {
            let reg = crate::model::REGISTRY.lock().unwrap();
            let info = reg.get(crate::model::ModelId::SPEAKER_ERES2NET).unwrap();
            info.data.to_vec()
        };
        let session = env.new_session(&model_data).unwrap();

        // Input: [1, T=40, 80] fbank features (matching Go test).
        let t = 40i64;
        let mels = 80i64;
        let mut data = vec![0.0f32; (1 * t * mels) as usize];
        for (i, v) in data.iter_mut().enumerate() {
            *v = (i % 100) as f32 * 0.01;
        }
        let input = Tensor::new(&[1, t, mels], &data).unwrap();

        let outputs = session.run(&["x"], &[&input], &["embedding"]).unwrap();
        assert_eq!(outputs.len(), 1);

        let embedding = outputs[0].float_data().unwrap();
        let shape = outputs[0].shape().unwrap();
        // Speaker model should output [1, 512] embedding.
        assert_eq!(shape.len(), 2);
        assert_eq!(shape[1], 512);
        assert_eq!(embedding.len(), 512);

        // No NaN or Inf.
        for (i, &v) in embedding.iter().enumerate() {
            assert!(!v.is_nan() && !v.is_infinite(), "emb[{i}] = {v}");
        }
    }
}
