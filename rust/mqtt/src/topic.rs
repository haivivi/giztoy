//! Topic utilities for MQTT.
//!
//! Provides `TopicWriter` and `TopicSubscriber` for convenient topic operations.

use crate::client::Conn;
use crate::error::{Error, Result};
use crate::types::QoS;
use parking_lot::RwLock;
use std::io::{self, Write};
use std::sync::Arc;
use std::time::{Duration, Instant};

/// Options for writing to a topic.
#[derive(Clone, Default)]
pub struct TopicWriteOptions {
    /// QoS level.
    pub qos: QoS,
    /// Retain flag.
    pub retain: bool,
}

impl TopicWriteOptions {
    /// Create new options with QoS.
    pub fn with_qos(mut self, qos: QoS) -> Self {
        self.qos = qos;
        self
    }

    /// Set retain flag.
    pub fn with_retain(mut self) -> Self {
        self.retain = true;
        self
    }
}

/// MQTT topic writer.
///
/// Implements `std::io::Write` for convenient writing to a topic.
pub struct TopicWriter {
    /// Topic name.
    pub name: String,
    /// Write options.
    pub options: TopicWriteOptions,
    /// Connection.
    conn: Arc<Conn>,
    /// Write deadline.
    write_deadline: RwLock<Option<Instant>>,
    /// Runtime handle for blocking write.
    runtime: tokio::runtime::Handle,
}

impl TopicWriter {
    /// Create a new topic writer.
    pub fn new(conn: Arc<Conn>, name: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            options: TopicWriteOptions::default(),
            conn,
            write_deadline: RwLock::new(None),
            runtime: tokio::runtime::Handle::current(),
        }
    }

    /// Create a new topic writer with options.
    pub fn with_options(mut self, options: TopicWriteOptions) -> Self {
        self.options = options;
        self
    }

    /// Set the write deadline.
    pub fn set_write_deadline(&self, deadline: Option<Instant>) {
        *self.write_deadline.write() = deadline;
    }

    /// Set the write timeout (convenience method).
    pub fn set_write_timeout(&self, timeout: Duration) {
        *self.write_deadline.write() = Some(Instant::now() + timeout);
    }

    /// Publish a message to the topic.
    pub async fn publish(&self, payload: &[u8]) -> Result<()> {
        // Check deadline
        if let Some(deadline) = *self.write_deadline.read() {
            if Instant::now() > deadline {
                return Err(Error::WriteDeadlineExceeded);
            }
        }

        let opts = if self.options.retain {
            vec![
                crate::client::WriteOption::Qos(self.options.qos),
                crate::client::WriteOption::Retain,
            ]
        } else {
            vec![crate::client::WriteOption::Qos(self.options.qos)]
        };

        self.conn
            .write_to_topic_with_opts(payload, &self.name, &opts)
            .await
    }
}

impl Write for TopicWriter {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        // Check deadline
        if let Some(deadline) = *self.write_deadline.read() {
            if Instant::now() > deadline {
                return Err(io::Error::new(
                    io::ErrorKind::TimedOut,
                    Error::WriteDeadlineExceeded,
                ));
            }
        }

        let conn = self.conn.clone();
        let topic = self.name.clone();
        let payload = buf.to_vec();
        let qos = self.options.qos;
        let retain = self.options.retain;

        self.runtime.block_on(async move {
            let opts = if retain {
                vec![
                    crate::client::WriteOption::Qos(qos),
                    crate::client::WriteOption::Retain,
                ]
            } else {
                vec![crate::client::WriteOption::Qos(qos)]
            };

            conn.write_to_topic_with_opts(&payload, &topic, &opts)
                .await
                .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;

            Ok(payload.len())
        })
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

/// MQTT topic subscriber.
///
/// Provides convenient subscription management.
pub struct TopicSubscriber {
    /// Topic pattern.
    pub name: String,
    /// Subscription options.
    pub qos: QoS,
    /// Shared group (if any).
    pub shared_group: Option<String>,
    /// Auto resubscribe on reconnect.
    pub auto_resubscribe: bool,
    /// Connection.
    conn: Arc<Conn>,
}

impl TopicSubscriber {
    /// Create a new topic subscriber.
    pub fn new(conn: Arc<Conn>, name: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            qos: QoS::AtMostOnce,
            shared_group: None,
            auto_resubscribe: false,
            conn,
        }
    }

    /// Set QoS level.
    pub fn with_qos(mut self, qos: QoS) -> Self {
        self.qos = qos;
        self
    }

    /// Set shared group.
    pub fn with_shared_group(mut self, group: impl Into<String>) -> Self {
        self.shared_group = Some(group.into());
        self
    }

    /// Enable auto resubscribe.
    pub fn with_auto_resubscribe(mut self) -> Self {
        self.auto_resubscribe = true;
        self
    }

    /// Subscribe to the topic.
    pub async fn subscribe(&self) -> Result<()> {
        let mut opts = vec![crate::client::SubscribeOption::Qos(self.qos)];

        if let Some(ref group) = self.shared_group {
            opts.push(crate::client::SubscribeOption::SharedGroup(group.clone()));
        }

        if self.auto_resubscribe {
            opts.push(crate::client::SubscribeOption::AutoResubscribe);
        }

        self.conn.subscribe_with_opts(&self.name, &opts).await
    }

    /// Unsubscribe from the topic.
    pub async fn unsubscribe(&self) -> Result<()> {
        self.conn.unsubscribe(&self.name).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_topic_write_options() {
        let opts = TopicWriteOptions::default()
            .with_qos(QoS::AtLeastOnce)
            .with_retain();

        assert_eq!(opts.qos, QoS::AtLeastOnce);
        assert!(opts.retain);
    }
}
