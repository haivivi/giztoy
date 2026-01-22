//! Trie data structure for MQTT topic pattern matching.
//!
//! Supports MQTT wildcards:
//! - `+` matches exactly one topic level
//! - `#` matches any number of remaining topic levels (must be last)

use parking_lot::RwLock;
use std::collections::HashMap;
use std::fmt;

use crate::error::{Error, Result};

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
            Some(idx) => (&pattern[..idx], &pattern[idx + 1..]),
        };

        match first {
            "+" => {
                if self.match_any.is_none() {
                    self.match_any = Some(Box::new(TrieNode::new()));
                }
                self.match_any.as_mut().unwrap().set(subseq, f)
            }
            "#" => {
                if !subseq.is_empty() {
                    return Err(Error::InvalidConfig("# must be the last segment".to_string()));
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

    /// Get values for the given topic.
    pub fn get(&self, topic: &str) -> Vec<&T> {
        if topic.is_empty() {
            return self.values.iter().collect();
        }

        let (first, subseq) = match topic.find('/') {
            None => (topic, ""),
            Some(idx) => (&topic[..idx], &topic[idx + 1..]),
        };

        let mut results = Vec::new();

        // Try exact match
        if let Some(child) = self.children.get(first) {
            results.extend(child.get(subseq));
        }

        // Try single-level wildcard (+)
        if let Some(ref match_any) = self.match_any {
            results.extend(match_any.get(subseq));
        }

        // Try multi-level wildcard (#)
        if let Some(ref match_all) = self.match_all {
            results.extend(match_all.values.iter());
        }

        results
    }

    /// Remove values matching the predicate.
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

    /// Remove values matching the predicate.
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
        trie.insert("device/gear-001/state", "handler1".to_string()).unwrap();

        let values = trie.get("device/gear-001/state");
        assert_eq!(values.len(), 1);
        assert_eq!(values[0], "handler1");

        assert!(trie.get("device/gear-002/state").is_empty());
    }

    #[test]
    fn test_single_level_wildcard() {
        let trie: Trie<String> = Trie::new();
        trie.insert("device/+/state", "wildcard".to_string()).unwrap();

        assert!(!trie.get("device/gear-001/state").is_empty());
        assert!(!trie.get("device/gear-002/state").is_empty());
        assert!(trie.get("device/state").is_empty());
    }

    #[test]
    fn test_multi_level_wildcard() {
        let trie: Trie<String> = Trie::new();
        trie.insert("device/#", "multi".to_string()).unwrap();

        assert!(!trie.get("device/gear-001").is_empty());
        assert!(!trie.get("device/gear-001/state").is_empty());
        assert!(trie.get("other/gear-001").is_empty());
    }
}
