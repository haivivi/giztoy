//! Common types for mqtt0.

use bytes::Bytes;

/// MQTT protocol version.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ProtocolVersion {
    /// MQTT 3.1.1
    #[default]
    V4,
    /// MQTT 5.0
    V5,
}

impl std::fmt::Display for ProtocolVersion {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ProtocolVersion::V4 => write!(f, "MQTT 3.1.1"),
            ProtocolVersion::V5 => write!(f, "MQTT 5.0"),
        }
    }
}

/// Quality of Service level.
///
/// This crate only supports QoS 0, but we keep the enum for API consistency.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum QoS {
    /// At most once delivery (fire and forget).
    #[default]
    AtMostOnce = 0,
}

impl From<rumqttc::mqttbytes::QoS> for QoS {
    fn from(qos: rumqttc::mqttbytes::QoS) -> Self {
        match qos {
            rumqttc::mqttbytes::QoS::AtMostOnce => QoS::AtMostOnce,
            _ => QoS::AtMostOnce, // Downgrade to QoS 0
        }
    }
}

impl From<QoS> for rumqttc::mqttbytes::QoS {
    fn from(_: QoS) -> Self {
        rumqttc::mqttbytes::QoS::AtMostOnce
    }
}

/// MQTT message.
#[derive(Debug, Clone)]
pub struct Message {
    /// Topic name.
    pub topic: String,
    /// Message payload.
    pub payload: Bytes,
    /// Retain flag.
    pub retain: bool,
}

impl Message {
    /// Create a new message.
    pub fn new(topic: impl Into<String>, payload: impl Into<Bytes>) -> Self {
        Self {
            topic: topic.into(),
            payload: payload.into(),
            retain: false,
        }
    }

    /// Set retain flag.
    pub fn with_retain(mut self, retain: bool) -> Self {
        self.retain = retain;
        self
    }
}

/// Authentication and authorization for MQTT clients.
pub trait Authenticator: Send + Sync {
    /// Authenticate a client connection.
    ///
    /// Called when a client sends CONNECT packet.
    /// Returns true to allow the connection.
    fn authenticate(&self, client_id: &str, username: &str, password: &[u8]) -> bool;

    /// Check ACL permissions.
    ///
    /// Called when a client publishes or subscribes.
    /// - `write=true`: client is publishing to the topic
    /// - `write=false`: client is subscribing to the topic
    ///
    /// Returns true to allow the operation.
    fn acl(&self, client_id: &str, topic: &str, write: bool) -> bool;
}

/// Allow-all authenticator (default).
#[derive(Debug, Default, Clone)]
pub struct AllowAll;

impl Authenticator for AllowAll {
    fn authenticate(&self, _client_id: &str, _username: &str, _password: &[u8]) -> bool {
        true
    }

    fn acl(&self, _client_id: &str, _topic: &str, _write: bool) -> bool {
        true
    }
}

/// Message handler trait.
pub trait Handler: Send + Sync {
    /// Handle an incoming message.
    ///
    /// This is called for every message received by the broker,
    /// after it has been routed to subscribers.
    fn handle(&self, client_id: &str, msg: &Message);
}

/// Function-based handler.
impl<F> Handler for F
where
    F: Fn(&str, &Message) + Send + Sync,
{
    fn handle(&self, client_id: &str, msg: &Message) {
        self(client_id, msg)
    }
}
