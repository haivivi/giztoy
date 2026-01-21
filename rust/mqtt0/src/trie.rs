//! Trie data structure for MQTT topic pattern matching.
//!
//! Supports MQTT wildcards:
//! - `+` matches exactly one topic level
//! - `#` matches any number of remaining topic levels (must be last)

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::HashMap;
use std::fmt;

/// Trie node for MQTT topic pattern matching.
pub struct TrieNode<T> {
    children: HashMap<String, TrieNode<T>>,
    match_any: Option<Box<TrieNode<T>>>, // + wildcard
    match_all: Option<Box<TrieNode<T>>>, // # wildcard
    values: Vec<T>,
}

impl<T> Default for TrieNode<T> {
    fn default() -> Self {
        Self::new()
    }
}

impl<T> TrieNode<T> {
    /// Create a new empty trie node.
    pub fn new() -> Self {
        Self {
            children: HashMap::new(),
            match_any: None,
            match_all: None,
            values: Vec::new(),
        }
    }

    /// Insert a value at the given pattern.
    pub fn insert(&mut self, pattern: &str, value: T) -> Result<()> {
        self.set(pattern, |node| node.values.push(value))
    }

    /// Set a value using a closure at the given pattern.
    pub fn set<F>(&mut self, pattern: &str, f: F) -> Result<()>
    where
        F: FnOnce(&mut TrieNode<T>),
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
                            return Err(Error::InvalidConfig(
                                "invalid $share subscription".to_string(),
                            ));
                        }
                        let first = parts[1];
                        let subseq = if parts.len() == 3 { parts[2] } else { "" };
                        return self.set_internal(first, subseq, f);
                    }
                    "$queue" => {
                        // $queue/<first>/<subseq>
                        let parts: Vec<&str> = rest.splitn(2, '/').collect();
                        if parts.is_empty() {
                            return Err(Error::InvalidConfig(
                                "invalid $queue subscription".to_string(),
                            ));
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
        F: FnOnce(&mut TrieNode<T>),
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
                    return Err(Error::InvalidConfig(
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

    /// Get values for the given topic (exact match).
    pub fn get(&self, topic: &str) -> Vec<&T> {
        let (_, values, ok) = self.match_topic(topic);
        if ok {
            values
        } else {
            Vec::new()
        }
    }

    /// Match a topic and return the matched values.
    pub fn match_topic(&self, topic: &str) -> (String, Vec<&T>, bool) {
        self.match_topic_internal("", topic)
    }

    fn match_topic_internal(&self, matched: &str, topic: &str) -> (String, Vec<&T>, bool) {
        if topic.is_empty() {
            return (
                matched.to_string(),
                self.values.iter().collect(),
                !self.values.is_empty(),
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
            let (route, values, ok) = child.match_topic_internal(&new_matched, subseq);
            if ok {
                return (route, values, true);
            }
        }

        // Try single-level wildcard (+)
        if let Some(ref match_any) = self.match_any {
            let new_matched = if matched.is_empty() {
                "+".to_string()
            } else {
                format!("{}/+", matched)
            };
            let (route, values, ok) = match_any.match_topic_internal(&new_matched, subseq);
            if ok {
                return (route, values, true);
            }
        }

        // Try multi-level wildcard (#)
        if let Some(ref match_all) = self.match_all {
            let new_matched = if matched.is_empty() {
                "#".to_string()
            } else {
                format!("{}/#", matched)
            };
            let (route, values, ok) = match_all.match_topic_internal(&new_matched, "");
            if ok {
                return (route, values, true);
            }
        }

        (String::new(), Vec::new(), false)
    }

    /// Remove a specific value from the given pattern.
    pub fn remove<F>(&mut self, pattern: &str, predicate: F) -> bool
    where
        F: Fn(&T) -> bool,
    {
        if pattern.is_empty() {
            let len_before = self.values.len();
            self.values.retain(|v| !predicate(v));
            return self.values.len() < len_before;
        }

        let (first, subseq) = match pattern.find('/') {
            None => (pattern, ""),
            Some(idx) => (&pattern[..idx], &pattern[idx + 1..]),
        };

        match first {
            "+" => {
                if let Some(ref mut match_any) = self.match_any {
                    return match_any.remove(subseq, predicate);
                }
            }
            "#" => {
                if let Some(ref mut match_all) = self.match_all {
                    let len_before = match_all.values.len();
                    match_all.values.retain(|v| !predicate(v));
                    return match_all.values.len() < len_before;
                }
            }
            _ => {
                if let Some(child) = self.children.get_mut(first) {
                    return child.remove(subseq, predicate);
                }
            }
        }

        false
    }

    /// Add a value to this node.
    pub fn add_value(&mut self, value: T) {
        self.values.push(value);
    }

    /// Get all values at this node.
    pub fn values(&self) -> &[T] {
        &self.values
    }
}

impl<T: fmt::Debug> fmt::Debug for TrieNode<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("TrieNode")
            .field("children", &self.children.keys().collect::<Vec<_>>())
            .field("match_any", &self.match_any.is_some())
            .field("match_all", &self.match_all.is_some())
            .field("values", &self.values.len())
            .finish()
    }
}

/// Thread-safe Trie for MQTT topic pattern matching.
pub struct Trie<T> {
    root: RwLock<TrieNode<T>>,
}

impl<T> Default for Trie<T> {
    fn default() -> Self {
        Self::new()
    }
}

impl<T> Trie<T> {
    /// Create a new empty trie.
    pub fn new() -> Self {
        Self {
            root: RwLock::new(TrieNode::new()),
        }
    }

    /// Insert a value at the given pattern.
    pub fn insert(&self, pattern: &str, value: T) -> Result<()> {
        self.root.write().insert(pattern, value)
    }

    /// Get values matching the given topic.
    pub fn get(&self, topic: &str) -> Vec<T>
    where
        T: Clone,
    {
        self.root.read().get(topic).into_iter().cloned().collect()
    }

    /// Match a topic and return the matched route and values.
    pub fn match_topic(&self, topic: &str) -> (String, Vec<T>, bool)
    where
        T: Clone,
    {
        let guard = self.root.read();
        let (route, values, ok) = guard.match_topic(topic);
        (route, values.into_iter().cloned().collect(), ok)
    }

    /// Remove values matching the predicate from the given pattern.
    pub fn remove<F>(&self, pattern: &str, predicate: F) -> bool
    where
        F: Fn(&T) -> bool,
    {
        self.root.write().remove(pattern, predicate)
    }

    /// Execute a function with mutable access to the root node.
    pub fn with_mut<F, R>(&self, f: F) -> R
    where
        F: FnOnce(&mut TrieNode<T>) -> R,
    {
        f(&mut self.root.write())
    }
}

impl<T: fmt::Debug> fmt::Debug for Trie<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self.root.read())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_exact_match() {
        let trie: Trie<String> = Trie::new();
        trie.insert("device/gear-001/state", "handler1".to_string())
            .unwrap();

        // Should match exact topic
        let values = trie.get("device/gear-001/state");
        assert_eq!(values.len(), 1);
        assert_eq!(values[0], "handler1");

        // Should not match different topic
        assert!(trie.get("device/gear-002/state").is_empty());

        // Should not match partial topic
        assert!(trie.get("device/gear-001").is_empty());
    }

    #[test]
    fn test_single_level_wildcard() {
        let trie: Trie<String> = Trie::new();
        trie.insert("device/+/state", "wildcard".to_string())
            .unwrap();

        // Should match any single level
        assert!(!trie.get("device/gear-001/state").is_empty());
        assert!(!trie.get("device/gear-002/state").is_empty());
        assert!(!trie.get("device/abc/state").is_empty());

        // Should not match
        assert!(trie.get("device/state").is_empty()); // Missing middle level
        assert!(trie.get("device/a/b/state").is_empty()); // Too many levels
    }

    #[test]
    fn test_multi_level_wildcard() {
        let trie: Trie<String> = Trie::new();
        trie.insert("device/#", "multi".to_string()).unwrap();

        // Should match any number of levels after device/
        assert!(!trie.get("device/gear-001").is_empty());
        assert!(!trie.get("device/gear-001/state").is_empty());
        assert!(!trie.get("device/gear-001/state/value").is_empty());

        // Should not match
        assert!(trie.get("other/gear-001").is_empty()); // Wrong prefix
    }

    #[test]
    fn test_multi_level_wildcard_must_be_last() {
        let trie: Trie<String> = Trie::new();
        let result = trie.insert("device/#/state", "invalid".to_string());
        assert!(result.is_err());
    }

    #[test]
    fn test_remove() {
        let trie: Trie<String> = Trie::new();
        trie.insert("device/+/state", "handler1".to_string())
            .unwrap();
        trie.insert("device/+/state", "handler2".to_string())
            .unwrap();

        assert_eq!(trie.get("device/gear-001/state").len(), 2);

        // Remove handler1
        trie.remove("device/+/state", |v| v == "handler1");
        let values = trie.get("device/gear-001/state");
        assert_eq!(values.len(), 1);
        assert_eq!(values[0], "handler2");
    }
}
