//! Error types for the MQTT library.

use thiserror::Error;

/// Error type for MQTT operations.
#[derive(Error, Debug)]
pub enum Error {
    /// Server is closed.
    #[error("mqtt: server closed")]
    ServerClosed,

    /// Server is already running.
    #[error("mqtt: server already running")]
    ServerRunning,

    /// Server is not running.
    #[error("mqtt: server not running")]
    ServerNotRunning,

    /// Invalid topic pattern.
    #[error("mqtt: invalid topic pattern: {0}")]
    InvalidTopicPattern(String),

    /// Invalid share subscription.
    #[error("mqtt: invalid share subscription")]
    InvalidShareSubscription,

    /// Invalid queue subscription.
    #[error("mqtt: invalid queue subscription")]
    InvalidQueueSubscription,

    /// No handler found for topic.
    #[error("mqtt: no handler found for topic: {0}")]
    NoHandlerFound(String),

    /// Connection error.
    #[error("mqtt: connection error: {0}")]
    Connection(String),

    /// Publish error.
    #[error("mqtt: publish error: {0}")]
    Publish(String),

    /// Subscribe error.
    #[error("mqtt: subscribe error: {0}")]
    Subscribe(String),

    /// Client error from rumqttc.
    #[error("mqtt client error: {0}")]
    ClientError(#[from] rumqttc::ClientError),

    /// Connection error from rumqttc.
    #[error("mqtt connection error: {0}")]
    ConnectionError(#[from] rumqttc::ConnectionError),

    /// IO error.
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),

    /// Write deadline exceeded.
    #[error("mqtt: write deadline exceeded")]
    WriteDeadlineExceeded,

    /// Nil topic writer.
    #[error("mqtt: write to nil topic writer")]
    NilTopicWriter,

    /// Handler error.
    #[error("mqtt: handler error: {0}")]
    Handler(String),
}

/// Result type for MQTT operations.
pub type Result<T> = std::result::Result<T, Error>;
