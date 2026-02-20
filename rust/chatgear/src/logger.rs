//! Logging interface for chatgear.
//!
//! Provides a `Logger` trait and a default implementation using the `tracing` crate.

use std::fmt;
use std::sync::Arc;

/// Logger interface for chatgear components.
///
/// All chatgear modules accept a `Logger` for structured diagnostic output.
/// The default implementation forwards to the `tracing` crate.
pub trait Logger: Send + Sync {
    fn error(&self, msg: &str);
    fn warn(&self, msg: &str);
    fn info(&self, msg: &str);
    fn debug(&self, msg: &str);
}

/// Returns the default logger that uses the `tracing` crate.
pub fn default_logger() -> Arc<dyn Logger> {
    Arc::new(TracingLogger)
}

/// Default logger implementation using `tracing`.
struct TracingLogger;

impl Logger for TracingLogger {
    fn error(&self, msg: &str) {
        tracing::error!("chatgear: {}", msg);
    }

    fn warn(&self, msg: &str) {
        tracing::warn!("chatgear: {}", msg);
    }

    fn info(&self, msg: &str) {
        tracing::info!("chatgear: {}", msg);
    }

    fn debug(&self, msg: &str) {
        tracing::debug!("chatgear: {}", msg);
    }
}

/// No-op logger that discards all messages.
pub struct NopLogger;

impl Logger for NopLogger {
    fn error(&self, _msg: &str) {}
    fn warn(&self, _msg: &str) {}
    fn info(&self, _msg: &str) {}
    fn debug(&self, _msg: &str) {}
}

/// Convenience macro for formatted error logging.
#[macro_export]
macro_rules! log_error {
    ($logger:expr, $($arg:tt)*) => {
        $logger.error(&format!($($arg)*))
    };
}

/// Convenience macro for formatted warn logging.
#[macro_export]
macro_rules! log_warn {
    ($logger:expr, $($arg:tt)*) => {
        $logger.warn(&format!($($arg)*))
    };
}

/// Convenience macro for formatted info logging.
#[macro_export]
macro_rules! log_info {
    ($logger:expr, $($arg:tt)*) => {
        $logger.info(&format!($($arg)*))
    };
}

/// Convenience macro for formatted debug logging.
#[macro_export]
macro_rules! log_debug {
    ($logger:expr, $($arg:tt)*) => {
        $logger.debug(&format!($($arg)*))
    };
}

/// Creates a formatted error as `std::io::Error`, matching Go's `Logger.Errorf`.
pub fn errorf(msg: impl fmt::Display) -> std::io::Error {
    std::io::Error::new(std::io::ErrorKind::Other, format!("chatgear: {}", msg))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    struct CapturingLogger {
        messages: Mutex<Vec<(String, String)>>,
    }

    impl CapturingLogger {
        fn new() -> Self {
            Self {
                messages: Mutex::new(Vec::new()),
            }
        }

        fn messages(&self) -> Vec<(String, String)> {
            self.messages.lock().unwrap().clone()
        }
    }

    impl Logger for CapturingLogger {
        fn error(&self, msg: &str) {
            self.messages.lock().unwrap().push(("error".to_string(), msg.to_string()));
        }
        fn warn(&self, msg: &str) {
            self.messages.lock().unwrap().push(("warn".to_string(), msg.to_string()));
        }
        fn info(&self, msg: &str) {
            self.messages.lock().unwrap().push(("info".to_string(), msg.to_string()));
        }
        fn debug(&self, msg: &str) {
            self.messages.lock().unwrap().push(("debug".to_string(), msg.to_string()));
        }
    }

    #[test]
    fn test_capturing_logger() {
        let logger = CapturingLogger::new();
        log_error!(logger, "test error {}", 42);
        log_warn!(logger, "test warn");
        log_info!(logger, "connected to {}", "device-001");
        log_debug!(logger, "frame len={}", 320);

        let msgs = logger.messages();
        assert_eq!(msgs.len(), 4);
        assert_eq!(msgs[0], ("error".to_string(), "test error 42".to_string()));
        assert_eq!(msgs[1], ("warn".to_string(), "test warn".to_string()));
        assert_eq!(msgs[2], ("info".to_string(), "connected to device-001".to_string()));
        assert_eq!(msgs[3], ("debug".to_string(), "frame len=320".to_string()));
    }

    #[test]
    fn test_nop_logger() {
        let logger = NopLogger;
        logger.error("should not panic");
        logger.warn("should not panic");
        logger.info("should not panic");
        logger.debug("should not panic");
    }

    #[test]
    fn test_errorf() {
        let err = errorf("connection timeout");
        assert!(err.to_string().contains("chatgear: connection timeout"));
    }

    #[test]
    fn test_default_logger() {
        let logger = default_logger();
        logger.info("test default logger");
    }
}
