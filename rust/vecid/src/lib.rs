//! Stable vector ID assignment via online nearest-centroid matching
//! and offline DBSCAN re-clustering.
//!
//! This crate is generic — works with any embedding type (voice, face, text, etc.).
//!
//! # Usage
//!
//! ```
//! use giztoy_vecid::{Config, Registry, MemoryStore};
//!
//! let store = MemoryStore::new();
//! let reg = Registry::new(
//!     Config { dim: 512, threshold: 0.5, min_samples: 2, prefix: "speaker".into() },
//!     Box::new(store),
//! );
//!
//! // Online: returns best guess (may be unmatched until first Recluster)
//! let (id, conf, matched) = reg.identify(&embedding);
//!
//! // Offline: re-cluster all stored embeddings for better accuracy
//! let n = reg.recluster();
//! ```
//!
//! # Design
//!
//! [`Registry::identify`] does NOT create new buckets — only [`Registry::recluster`] does.
//! This avoids the greedy merge / chain drift problem where sequential merging
//! causes order-dependent results and cluster quality degradation.

mod dbscan;
mod error;
mod store;
mod vecid;

pub use error::VecIdError;
pub use store::{MemoryStore, VecIdStore};
pub use vecid::{Bucket, Config, Registry};
