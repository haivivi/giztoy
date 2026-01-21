//! Trie data structure for MQTT topic pattern matching.
//!
//! Supports MQTT wildcards:
//! - `+` matches exactly one topic level
//! - `#` matches any number of remaining topic levels (must be last)

use crate::error::{Error, Result};
use crate::serve_mux::Handler;
use parking_lot::RwLock;
use std::collections::HashMap;
use std::fmt;
use std::sync::Arc;

/// Trie node for MQTT topic pattern matching.
pub struct TrieNode {
    children: HashMap<String, TrieNode>,
    match_any: Option<Box<TrieNode>>, // + wildcard
    match_all: Option<Box<TrieNode>>, // # wildcard
    handlers: Vec<Arc<dyn Handler>>,
}

impl Default for TrieNode {
    fn default() -> Self {
        Self::new()
    }
}

impl TrieNode {
    /// Create a new empty trie node.
    pub fn new() -> Self {
        Self {
            children: HashMap::new(),
            match_any: None,
            match_all: None,
            handlers: Vec::new(),
        }
    }

    /// Set a handler for the given pattern.
    pub fn set<F>(&mut self, pattern: &str, f: F) -> Result<()>
    where
        F: FnOnce(&mut TrieNode),
    {
        if pattern.is_empty() {
            f(self);
            return Ok(());
        }

        let (first, subseq) = match pattern.find('/') {
            None => (pattern, ""),
            Some(idx) => {
                let first = &pattern[..idx];
                let rest = &pattern[idx + 1..];

                match first {
                    "$share" => {
                        // $share/<sharename>/<first>/<subseq>
                        let parts: Vec<&str> = rest.splitn(3, '/').collect();
                        if parts.len() < 2 {
                            return Err(Error::InvalidShareSubscription);
                        }
                        let first = parts[1];
                        let subseq = if parts.len() == 3 { parts[2] } else { "" };
                        return self.set_internal(first, subseq, f);
                    }
                    "$queue" => {
                        // $queue/<first>/<subseq>
                        let parts: Vec<&str> = rest.splitn(2, '/').collect();
                        if parts.is_empty() {
                            return Err(Error::InvalidQueueSubscription);
                        }
                        let first = parts[0];
                        let subseq = if parts.len() == 2 { parts[1] } else { "" };
                        return self.set_internal(first, subseq, f);
                    }
                    _ => (first, rest),
                }
            }
        };

        self.set_internal(first, subseq, f)
    }

    fn set_internal<F>(&mut self, first: &str, subseq: &str, f: F) -> Result<()>
    where
        F: FnOnce(&mut TrieNode),
    {
        // Check existing children first
        if let Some(child) = self.children.get_mut(first) {
            return child.set(subseq, f);
        }

        match first {
            "+" => {
                // Single level wildcard
                if self.match_any.is_none() {
                    self.match_any = Some(Box::new(TrieNode::new()));
                }
                self.match_any.as_mut().unwrap().set(subseq, f)
            }
            "#" => {
                // Multi-level wildcard - must be last
                if !subseq.is_empty() {
                    return Err(Error::InvalidTopicPattern(
                        "# must be the last segment".to_string(),
                    ));
                }
                if self.match_all.is_none() {
                    self.match_all = Some(Box::new(TrieNode::new()));
                }
                f(self.match_all.as_mut().unwrap());
                Ok(())
            }
            _ => {
                let child = self.children.entry(first.to_string()).or_default();
                child.set(subseq, f)
            }
        }
    }

    /// Get handlers for the given topic.
    pub fn get(&self, topic: &str) -> Option<Vec<Arc<dyn Handler>>> {
        let (_, handlers, ok) = self.match_topic("", topic);
        if ok {
            Some(handlers)
        } else {
            None
        }
    }

    /// Match a topic and return the matched route, handlers, and success flag.
    pub fn match_topic(
        &self,
        matched: &str,
        topic: &str,
    ) -> (String, Vec<Arc<dyn Handler>>, bool) {
        if topic.is_empty() {
            return (
                matched.to_string(),
                self.handlers.clone(),
                !self.handlers.is_empty(),
            );
        }

        let (first, subseq) = match topic.find('/') {
            None => (topic, ""),
            Some(idx) => (&topic[..idx], &topic[idx + 1..]),
        };

        // Try exact match first
        if let Some(child) = self.children.get(first) {
            let new_matched = if matched.is_empty() {
                first.to_string()
            } else {
                format!("{}/{}", matched, first)
            };
            let (route, handlers, ok) = child.match_topic(&new_matched, subseq);
            if ok {
                return (route, handlers, true);
            }
        }

        // Try single-level wildcard (+)
        if let Some(ref match_any) = self.match_any {
            let new_matched = if matched.is_empty() {
                "+".to_string()
            } else {
                format!("{}/+", matched)
            };
            let (route, handlers, ok) = match_any.match_topic(&new_matched, subseq);
            if ok {
                return (route, handlers, true);
            }
        }

        // Try multi-level wildcard (#)
        if let Some(ref match_all) = self.match_all {
            let new_matched = if matched.is_empty() {
                "#".to_string()
            } else {
                format!("{}/#", matched)
            };
            let (route, handlers, ok) = match_all.match_topic(&new_matched, "");
            if ok {
                return (route, handlers, true);
            }
        }

        (String::new(), Vec::new(), false)
    }

    /// Walk the trie and call the function for each node with its path.
    pub fn walk_with_path<F>(&self, path: Vec<String>, f: &F)
    where
        F: Fn(&[String], &TrieNode),
    {
        for (seg, child) in &self.children {
            let mut new_path = path.clone();
            new_path.push(seg.clone());
            child.walk_with_path(new_path, f);
        }

        if let Some(ref match_any) = self.match_any {
            let mut new_path = path.clone();
            new_path.push("+".to_string());
            match_any.walk_with_path(new_path, f);
        }

        if let Some(ref match_all) = self.match_all {
            let mut new_path = path.clone();
            new_path.push("#".to_string());
            match_all.walk_with_path(new_path, f);
        }

        f(&path, self);
    }

    /// Add a handler to this node.
    pub fn add_handler(&mut self, handler: Arc<dyn Handler>) {
        self.handlers.push(handler);
    }
}

impl fmt::Debug for TrieNode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let lines = std::sync::Mutex::new(Vec::new());
        self.walk_with_path(Vec::new(), &|path, node| {
            lines.lock().unwrap().push(format!("{}: {} handlers", path.join("/"), node.handlers.len()));
        });
        let mut lines = lines.into_inner().unwrap();
        lines.sort();
        write!(f, "{}", lines.join("\n"))
    }
}

/// Thread-safe Trie for MQTT topic pattern matching.
pub struct Trie {
    root: RwLock<TrieNode>,
}

impl Default for Trie {
    fn default() -> Self {
        Self::new()
    }
}

impl Trie {
    /// Create a new empty trie.
    pub fn new() -> Self {
        Self {
            root: RwLock::new(TrieNode::new()),
        }
    }

    /// Set a handler for the given pattern.
    pub fn set<F>(&self, pattern: &str, f: F) -> Result<()>
    where
        F: FnOnce(&mut TrieNode),
    {
        self.root.write().set(pattern, f)
    }

    /// Get handlers for the given topic.
    pub fn get(&self, topic: &str) -> Option<Vec<Arc<dyn Handler>>> {
        self.root.read().get(topic)
    }

    /// Match a topic and return the matched route, handlers, and success flag.
    pub fn match_topic(&self, topic: &str) -> (String, Vec<Arc<dyn Handler>>, bool) {
        self.root.read().match_topic("", topic)
    }
}

impl fmt::Debug for Trie {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self.root.read())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::serve_mux::Message;

    struct MockHandler {
        name: String,
    }

    impl Handler for MockHandler {
        fn handle_message(&self, _msg: &Message) -> crate::Result<()> {
            Ok(())
        }
    }

    #[test]
    fn test_exact_match() {
        let trie = Trie::new();
        trie.set("device/gear-001/state", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "exact".to_string(),
            }));
        })
        .unwrap();

        // Should match exact topic
        assert!(trie.get("device/gear-001/state").is_some());

        // Should not match different topic
        assert!(trie.get("device/gear-002/state").is_none());

        // Should not match partial topic
        assert!(trie.get("device/gear-001").is_none());
    }

    #[test]
    fn test_single_level_wildcard() {
        let trie = Trie::new();
        trie.set("device/+/state", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "single-wildcard".to_string(),
            }));
        })
        .unwrap();

        // Should match any single level
        assert!(trie.get("device/gear-001/state").is_some());
        assert!(trie.get("device/gear-002/state").is_some());
        assert!(trie.get("device/abc/state").is_some());

        // Should not match
        assert!(trie.get("device/state").is_none()); // Missing middle level
        assert!(trie.get("device/a/b/state").is_none()); // Too many levels
        assert!(trie.get("other/gear-001/state").is_none()); // Wrong prefix
    }

    #[test]
    fn test_multi_level_wildcard() {
        let trie = Trie::new();
        trie.set("device/#", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "multi-wildcard".to_string(),
            }));
        })
        .unwrap();

        // Should match any number of levels after device/
        assert!(trie.get("device/gear-001").is_some());
        assert!(trie.get("device/gear-001/state").is_some());
        assert!(trie.get("device/gear-001/state/value").is_some());
        assert!(trie.get("device/a/b/c/d/e").is_some());

        // Should not match
        assert!(trie.get("other/gear-001").is_none()); // Wrong prefix
    }

    #[test]
    fn test_multi_level_wildcard_must_be_last() {
        let trie = Trie::new();

        // # must be the last segment
        let result = trie.set("device/#/state", |_node| {});
        assert!(result.is_err());
    }

    #[test]
    fn test_combined_wildcards() {
        let trie = Trie::new();
        trie.set("device/+/events/#", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "combined".to_string(),
            }));
        })
        .unwrap();

        // Should match
        assert!(trie.get("device/gear-001/events/click").is_some());
        assert!(trie.get("device/gear-002/events/touch/start").is_some());
        assert!(trie.get("device/abc/events/a/b/c").is_some());

        // Should not match
        assert!(trie.get("device/gear-001/state").is_none());
        assert!(trie.get("device/events/click").is_none());
        assert!(trie.get("device/a/b/events/click").is_none());
    }

    #[test]
    fn test_multiple_patterns() {
        let trie = Trie::new();

        let patterns = [
            "device/+/state",
            "device/+/stats",
            "device/+/input_audio_stream",
            "server/push/#",
        ];

        for pattern in &patterns {
            trie.set(pattern, |node| {
                node.add_handler(Arc::new(MockHandler {
                    name: pattern.to_string(),
                }));
            })
            .unwrap();
        }

        assert!(trie.get("device/gear-001/state").is_some());
        assert!(trie.get("device/gear-001/stats").is_some());
        assert!(trie.get("device/gear-001/input_audio_stream").is_some());
        assert!(trie.get("server/push/vd/mode_changed").is_some());
        assert!(trie.get("server/push/user/command").is_some());

        assert!(trie.get("device/gear-001/unknown").is_none());
        assert!(trie.get("other/topic").is_none());
    }

    #[test]
    fn test_share_subscription() {
        let trie = Trie::new();
        trie.set("$share/group1/device/+/state", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "shared".to_string(),
            }));
        })
        .unwrap();

        // Shared subscription should match the underlying topic pattern
        assert!(trie.get("device/gear-001/state").is_some());
        assert!(trie.get("device/gear-002/state").is_some());
        assert!(trie.get("device/gear-001/stats").is_none());
    }

    #[test]
    fn test_invalid_share_subscription() {
        let trie = Trie::new();

        // $share requires at least group name and topic
        let result = trie.set("$share/group1", |_node| {});
        assert!(result.is_err());
    }

    #[test]
    fn test_queue_subscription() {
        let trie = Trie::new();
        trie.set("$queue/device/+/state", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "queue".to_string(),
            }));
        })
        .unwrap();

        // Queue subscription should match the underlying topic pattern
        assert!(trie.get("device/gear-001/state").is_some());
    }

    #[test]
    fn test_empty_pattern() {
        let trie = Trie::new();
        trie.set("", |node| {
            node.add_handler(Arc::new(MockHandler {
                name: "root".to_string(),
            }));
        })
        .unwrap();

        // Empty topic should match empty pattern
        let handlers = trie.get("").unwrap();
        assert_eq!(handlers.len(), 1);
    }

    #[test]
    fn test_match_priority() {
        let trie = Trie::new();

        // Register patterns in different order
        let patterns = ["device/#", "device/+/state", "device/gear-001/state"];

        for pattern in &patterns {
            trie.set(pattern, |node| {
                node.add_handler(Arc::new(MockHandler {
                    name: pattern.to_string(),
                }));
            })
            .unwrap();
        }

        // Exact match should be returned first (based on trie traversal order)
        let handlers = trie.get("device/gear-001/state").unwrap();
        assert_eq!(handlers.len(), 1);
    }

    #[test]
    fn test_multiple_handlers_per_pattern() {
        let trie = Trie::new();

        // Add multiple handlers to the same pattern
        for _ in 0..3 {
            trie.set("device/+/state", |node| {
                node.add_handler(Arc::new(MockHandler {
                    name: "handler".to_string(),
                }));
            })
            .unwrap();
        }

        let handlers = trie.get("device/gear-001/state").unwrap();
        assert_eq!(handlers.len(), 3);
    }
}
