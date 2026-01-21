//! ServeMux for MQTT message routing.
//!
//! Provides a multiplexer that routes incoming MQTT messages to registered handlers
//! based on topic patterns.

use crate::error::{Error, Result};
use crate::trie::{Trie, TrieNode};
use bytes::Bytes;
use parking_lot::RwLock;
use std::collections::HashMap;
use std::fmt;
use std::sync::Arc;
use tracing::debug;

/// MQTT message received from a subscription.
#[derive(Debug, Clone)]
pub struct Message {
    /// Topic the message was published to.
    pub topic: String,
    /// Message payload.
    pub payload: Bytes,
    /// QoS level.
    pub qos: u8,
    /// Retain flag.
    pub retain: bool,
    /// Packet ID (for QoS > 0).
    pub packet_id: Option<u16>,
    /// User properties.
    pub user_properties: Vec<(String, String)>,
    /// Client ID (if available, set by server).
    pub client_id: Option<String>,
}

impl Message {
    /// Create a new message.
    pub fn new(topic: impl Into<String>, payload: impl Into<Bytes>) -> Self {
        Self {
            topic: topic.into(),
            payload: payload.into(),
            qos: 0,
            retain: false,
            packet_id: None,
            user_properties: Vec::new(),
            client_id: None,
        }
    }

    /// Get the payload as a string (if valid UTF-8).
    pub fn payload_str(&self) -> Option<&str> {
        std::str::from_utf8(&self.payload).ok()
    }
}

/// Handler trait for processing MQTT messages.
pub trait Handler: Send + Sync {
    /// Handle an incoming MQTT message.
    fn handle_message(&self, msg: &Message) -> Result<()>;
}

/// Handler function type.
pub type HandlerFunc = dyn Fn(&Message) -> Result<()> + Send + Sync;

/// Wrapper for handler functions.
struct FnHandler {
    f: Box<HandlerFunc>,
}

impl Handler for FnHandler {
    fn handle_message(&self, msg: &Message) -> Result<()> {
        (self.f)(msg)
    }
}

/// MQTT message multiplexer.
///
/// Routes incoming messages to handlers based on topic patterns.
/// Supports MQTT wildcards: `+` (single level) and `#` (multi-level).
pub struct ServeMux {
    trie: Trie,
    aliases: RwLock<HashMap<u16, String>>,
}

impl Default for ServeMux {
    fn default() -> Self {
        Self::new()
    }
}

impl ServeMux {
    /// Create a new empty ServeMux.
    pub fn new() -> Self {
        Self {
            trie: Trie::new(),
            aliases: RwLock::new(HashMap::new()),
        }
    }

    /// Register a handler function for the given pattern.
    ///
    /// # Example
    ///
    /// ```
    /// use giztoy_mqtt::ServeMux;
    ///
    /// let mux = ServeMux::new();
    /// mux.handle_func("device/+/state", |msg| {
    ///     println!("Received: {:?}", msg.payload);
    ///     Ok(())
    /// });
    /// ```
    pub fn handle_func<F>(&self, pattern: &str, f: F) -> Result<()>
    where
        F: Fn(&Message) -> Result<()> + Send + Sync + 'static,
    {
        self.trie.set(pattern, |node: &mut TrieNode| {
            node.add_handler(Arc::new(FnHandler { f: Box::new(f) }));
        })
    }

    /// Register a handler for the given pattern.
    ///
    /// # Example
    ///
    /// ```
    /// use giztoy_mqtt::{ServeMux, Handler, Message, Result};
    /// use std::sync::Arc;
    ///
    /// struct MyHandler;
    ///
    /// impl Handler for MyHandler {
    ///     fn handle_message(&self, msg: &Message) -> Result<()> {
    ///         println!("Received: {:?}", msg.payload);
    ///         Ok(())
    ///     }
    /// }
    ///
    /// let mux = ServeMux::new();
    /// mux.handle("device/+/state", Arc::new(MyHandler));
    /// ```
    pub fn handle(&self, pattern: &str, handler: Arc<dyn Handler>) -> Result<()> {
        self.trie.set(pattern, |node: &mut TrieNode| {
            node.add_handler(handler);
        })
    }

    /// Handle an incoming message by routing it to the appropriate handlers.
    pub fn handle_message(&self, msg: &Message) -> Result<()> {
        let topic = &msg.topic;

        debug!("handling message for topic: {}", topic);

        let (_, handlers, ok) = self.trie.match_topic(topic);
        if !ok {
            debug!("no handler found for topic: {}", topic);
            return Err(Error::NoHandlerFound(topic.clone()));
        }

        for handler in handlers {
            handler.handle_message(msg)?;
        }

        Ok(())
    }

    /// Register a topic alias.
    pub fn register_alias(&self, alias: u16, topic: String) {
        self.aliases.write().insert(alias, topic);
    }

    /// Resolve a topic alias.
    pub fn resolve_alias(&self, alias: u16) -> Option<String> {
        self.aliases.read().get(&alias).cloned()
    }

    /// Check if a pattern has handlers.
    pub fn has_handlers(&self, topic: &str) -> bool {
        self.trie.get(topic).is_some()
    }
}

impl fmt::Debug for ServeMux {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "ServeMux {{ trie: {:?} }}", self.trie)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    #[test]
    fn test_handle_func() {
        let mux = ServeMux::new();
        let counter = Arc::new(AtomicUsize::new(0));
        let counter_clone = counter.clone();

        mux.handle_func("test/topic", move |_msg| {
            counter_clone.fetch_add(1, Ordering::SeqCst);
            Ok(())
        })
        .unwrap();

        let msg = Message::new("test/topic", "hello");
        mux.handle_message(&msg).unwrap();

        assert_eq!(counter.load(Ordering::SeqCst), 1);
    }

    #[test]
    fn test_wildcard_handlers() {
        let mux = ServeMux::new();
        let counter = Arc::new(AtomicUsize::new(0));
        let counter_clone = counter.clone();

        mux.handle_func("device/+/state", move |_msg| {
            counter_clone.fetch_add(1, Ordering::SeqCst);
            Ok(())
        })
        .unwrap();

        // Should match
        mux.handle_message(&Message::new("device/gear-001/state", "data"))
            .unwrap();
        mux.handle_message(&Message::new("device/gear-002/state", "data"))
            .unwrap();

        assert_eq!(counter.load(Ordering::SeqCst), 2);

        // Should not match
        let result = mux.handle_message(&Message::new("device/gear-001/stats", "data"));
        assert!(result.is_err());
    }

    #[test]
    fn test_no_handler_error() {
        let mux = ServeMux::new();

        let result = mux.handle_message(&Message::new("nonexistent/topic", "data"));
        assert!(matches!(result, Err(Error::NoHandlerFound(_))));
    }

    #[test]
    fn test_topic_alias() {
        let mux = ServeMux::new();

        mux.register_alias(1, "device/gear-001/state".to_string());

        let resolved = mux.resolve_alias(1);
        assert_eq!(resolved, Some("device/gear-001/state".to_string()));

        let not_found = mux.resolve_alias(999);
        assert!(not_found.is_none());
    }
}
