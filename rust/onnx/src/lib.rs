//! Rust bindings for the ONNX Runtime C API.
//!
//! ONNX Runtime is a cross-platform inference engine for ONNX models.
//! This crate wraps the C API through a thin C shim, providing safe Rust
//! types for Environment, Session, and Tensor.
//!
//! # Usage
//!
//! ```no_run
//! use giztoy_onnx::{Env, Tensor, load_model};
//!
//! let env = Env::new("myapp").unwrap();
//! let session = load_model(&env, "speaker-eres2net").unwrap();
//!
//! let input = Tensor::new(&[1, 100, 80], &input_data).unwrap();
//! let outputs = session.run(&["feats"], &[&input], &["embs"]).unwrap();
//! let embedding = outputs[0].float_data().unwrap();
//! ```
//!
//! # Dynamic Linking
//!
//! ONNX Runtime is dynamically linked (.dylib/.so) via Bazel.
//! The pre-built library is downloaded from GitHub releases.

mod error;
mod ffi;
pub mod model;
mod model_embed;
mod onnx;

pub use error::OnnxError;
pub use model::{load_model, list_models, register_model, ModelId};
pub use model_embed::register_embedded_models;
pub use onnx::{Env, Session, Tensor};
