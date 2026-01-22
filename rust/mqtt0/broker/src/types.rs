//! Common types for mqtt0-broker.

use mqtt0::types::Message;

/// Authentication and authorization for MQTT clients.
pub trait Authenticator: Send + Sync {
    /// Authenticate a client connection.
    fn authenticate(&self, client_id: &str, username: &str, password: &[u8]) -> bool;

    /// Check ACL permissions.
    ///
    /// - `write=true`: client is publishing to the topic
    /// - `write=false`: client is subscribing to the topic
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
