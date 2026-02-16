//! Rust bindings for the ncnn neural network inference framework.
//!
//! ncnn is a high-performance inference framework optimized for mobile and
//! embedded platforms. This crate wraps the ncnn C API, providing safe Rust
//! types for Net (model), Extractor (inference session), and Mat (tensor).
//!
//! # Usage
//!
//! ```no_run
//! use giztoy_ncnn::{Net, NcnnOption, Mat};
//!
//! let mut opt = NcnnOption::new().unwrap();
//! opt.set_fp16(false);
//! let net = Net::from_memory(param_data, bin_data, Some(&opt)).unwrap();
//!
//! let mut ex = net.extractor().unwrap();
//! let input = Mat::new_2d(80, 300, &input_data).unwrap();
//! ex.set_input("in0", &input).unwrap();
//! let output = ex.extract("out0").unwrap();
//! let data = output.float_data().unwrap();
//! ```
//!
//! # Static Linking
//!
//! ncnn is statically linked (.a) into the binary via Bazel.
//! No external shared libraries are needed at runtime.
//!
//! # Thread Safety
//!
//! Net is safe for concurrent use — multiple Extractors can run in parallel
//! on the same Net. Each Extractor must be used from a single thread.

mod error;
mod ffi;
pub mod model;
mod model_embed;
mod ncnn;

pub use error::NcnnError;
pub use model::{load_model, list_models, register_model, ModelId};
pub use model_embed::register_embedded_models;
pub use ncnn::{version, Extractor, Mat, NcnnOption, Net};
