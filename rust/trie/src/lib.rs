//! Generic trie data structure for efficient path-based storage and retrieval.
//!
//! This crate provides a trie that supports MQTT-style topic patterns with wildcards:
//! - `/a/b/c` - exact path match
//! - `/a/+/c` - single-level wildcard (matches any single segment)
//! - `/a/#` - multi-level wildcard (matches any remaining path segments)
//!
//! The trie is useful for routing, topic matching, and hierarchical data storage.
//!
//! # Example
//!
//! ```rust
//! use giztoy_trie::Trie;
//!
//! let mut trie = Trie::<String>::new();
//!
//! // Set exact paths
//! trie.set_value("device/gear-001/state", "handler1".to_string()).unwrap();
//!
//! // Set wildcard patterns
//! trie.set_value("device/+/events", "handler2".to_string()).unwrap();
//! trie.set_value("logs/#", "handler3".to_string()).unwrap();
//!
//! // Get values
//! assert_eq!(trie.get_value("device/gear-001/state"), Some("handler1".to_string()));
//! assert_eq!(trie.get_value("device/any-device/events"), Some("handler2".to_string()));
//! assert_eq!(trie.get_value("logs/app/debug/line1"), Some("handler3".to_string()));
//! ```

use std::collections::HashMap;
use std::fmt;

/// Error returned when an invalid pattern is provided.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("invalid path pattern: path should be /a/b/c or /a/+/c or /a/#")]
pub struct InvalidPatternError;

/// A generic trie data structure that supports path-based storage with wildcard matching.
///
/// Uses MQTT-style topic patterns where:
/// - `/` separates path segments
/// - `+` matches any single segment (single-level wildcard)
/// - `#` matches any remaining path segments (multi-level wildcard)
#[derive(Debug, Clone)]
pub struct Trie<T> {
    children: HashMap<String, Trie<T>>,
    match_any: Option<Box<Trie<T>>>,  // single-level wildcard (+)
    match_all: Option<Box<Trie<T>>>,  // multi-level wildcard (#)
    value: Option<T>,
}

impl<T> Default for Trie<T> {
    fn default() -> Self {
        Self::new()
    }
}

impl<T> Trie<T> {
    /// Creates a new empty Trie.
    pub fn new() -> Self {
        Self {
            children: HashMap::new(),
            match_any: None,
            match_all: None,
            value: None,
        }
    }

    /// Stores a value at the specified path using the provided setter function.
    ///
    /// The setter function is called with a mutable reference to the current value
    /// (if any) and should return the new value or an error.
    ///
    /// # Path patterns
    /// - `a/b/c` - exact path segments
    /// - `a/b/c/` - same as `a/b/c`
    /// - `a/+/c` - single-level wildcard
    /// - `a/#` - multi-level wildcard (must be at end)
    ///
    /// # Errors
    /// Returns `InvalidPatternError` if `#` is not at the end of the path.
    pub fn set<F, E>(&mut self, path: &str, setter: F) -> Result<(), E>
    where
        F: FnOnce(Option<&mut T>) -> Result<T, E>,
    {
        self.set_internal(path, setter)
    }

    fn set_internal<F, E>(&mut self, path: &str, setter: F) -> Result<(), E>
    where
        F: FnOnce(Option<&mut T>) -> Result<T, E>,
    {
        if path.is_empty() {
            let new_value = setter(self.value.as_mut())?;
            self.value = Some(new_value);
            return Ok(());
        }

        let (first, rest) = split_path(path);

        match first {
            "+" => {
                if self.match_any.is_none() {
                    self.match_any = Some(Box::new(Trie::new()));
                }
                self.match_any.as_mut().unwrap().set_internal(rest, setter)
            }
            "#" => {
                if !rest.is_empty() {
                    return Err(setter(None).err().unwrap_or_else(|| {
                        panic!("# wildcard must be at the end of path")
                    }));
                }
                if self.match_all.is_none() {
                    self.match_all = Some(Box::new(Trie::new()));
                }
                let new_value = setter(self.match_all.as_mut().unwrap().value.as_mut())?;
                self.match_all.as_mut().unwrap().value = Some(new_value);
                Ok(())
            }
            _ => {
                if !self.children.contains_key(first) {
                    self.children.insert(first.to_string(), Trie::new());
                }
                self.children.get_mut(first).unwrap().set_internal(rest, setter)
            }
        }
    }

    /// Convenience method to store a value at the specified path.
    ///
    /// # Errors
    /// Returns `InvalidPatternError` if the pattern is invalid.
    pub fn set_value(&mut self, path: &str, value: T) -> Result<(), InvalidPatternError> {
        self.set(path, |_| Ok::<_, InvalidPatternError>(value))
    }

    /// Retrieves a reference to the value at the specified path.
    ///
    /// Returns `Some(&T)` if found, `None` otherwise.
    /// 
    /// This is a zero-allocation lookup operation.
    #[inline]
    pub fn get(&self, path: &str) -> Option<&T> {
        self.get_internal(path)
    }

    /// Internal get implementation - zero allocation path lookup.
    fn get_internal(&self, path: &str) -> Option<&T> {
        if path.is_empty() {
            return self.value.as_ref();
        }

        let (first, rest) = split_path(path);

        // Try exact match first (highest priority)
        if let Some(child) = self.children.get(first) {
            if let Some(val) = child.get_internal(rest) {
                return Some(val);
            }
        }

        // Try single-level wildcard
        if let Some(ref match_any) = self.match_any {
            if let Some(val) = match_any.get_internal(rest) {
                return Some(val);
            }
        }

        // Try multi-level wildcard
        if let Some(ref match_all) = self.match_all {
            if match_all.value.is_some() {
                return match_all.value.as_ref();
            }
        }

        None
    }

    /// Retrieves the value at the specified path.
    ///
    /// Returns `Some(T)` if found (cloned), `None` otherwise.
    pub fn get_value(&self, path: &str) -> Option<T>
    where
        T: Clone,
    {
        self.get(path).cloned()
    }

    /// Returns the matched route pattern and value for the given path.
    ///
    /// Returns `(route, Some(&value))` if found, `("", None)` otherwise.
    ///
    /// Note: This method allocates strings for the route. Use `get()` for 
    /// zero-allocation lookups when you only need the value.
    pub fn match_path(&self, path: &str) -> (String, Option<&T>) {
        let mut route_parts = Vec::new();
        if let Some(val) = self.match_with_route(&mut route_parts, path) {
            let route = if route_parts.is_empty() {
                String::new()
            } else {
                format!("/{}", route_parts.join("/"))
            };
            (route, Some(val))
        } else {
            (String::new(), None)
        }
    }

    /// Internal match implementation that builds the route.
    fn match_with_route<'a>(&'a self, route: &mut Vec<String>, path: &str) -> Option<&'a T> {
        if path.is_empty() {
            return self.value.as_ref();
        }

        let (first, rest) = split_path(path);

        // Try exact match first
        if let Some(child) = self.children.get(first) {
            let route_len = route.len();
            route.push(first.to_string());
            if let Some(val) = child.match_with_route(route, rest) {
                return Some(val);
            }
            route.truncate(route_len);
        }

        // Try single-level wildcard
        if let Some(ref match_any) = self.match_any {
            let route_len = route.len();
            route.push("+".to_string());
            if let Some(val) = match_any.match_with_route(route, rest) {
                return Some(val);
            }
            route.truncate(route_len);
        }

        // Try multi-level wildcard
        if let Some(ref match_all) = self.match_all {
            if match_all.value.is_some() {
                route.push("#".to_string());
                return match_all.value.as_ref();
            }
        }

        None
    }

    /// Walks the trie, calling the provided function for each node with a value.
    pub fn walk<F>(&self, mut f: F)
    where
        F: FnMut(&str, &T),
    {
        self.walk_internal(&mut Vec::new(), &mut f);
    }

    fn walk_internal<F>(&self, path: &mut Vec<String>, f: &mut F)
    where
        F: FnMut(&str, &T),
    {
        // Visit children (sorted for deterministic order)
        let mut keys: Vec<_> = self.children.keys().collect();
        keys.sort();
        for key in keys {
            path.push(key.clone());
            self.children[key].walk_internal(path, f);
            path.pop();
        }

        // Visit single-level wildcard
        if let Some(ref match_any) = self.match_any {
            path.push("+".to_string());
            match_any.walk_internal(path, f);
            path.pop();
        }

        // Visit multi-level wildcard
        if let Some(ref match_all) = self.match_all {
            path.push("#".to_string());
            match_all.walk_internal(path, f);
            path.pop();
        }

        // Visit this node
        if let Some(ref value) = self.value {
            f(&path.join("/"), value);
        }
    }

    /// Returns the number of values stored in the trie.
    pub fn len(&self) -> usize {
        let mut count = 0;
        self.walk(|_, _| count += 1);
        count
    }

    /// Returns true if the trie is empty.
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

impl<T: fmt::Display> fmt::Display for Trie<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mut lines = Vec::new();
        self.walk(|path, value| {
            lines.push(format!("{}: {}", path, value));
        });
        lines.sort();
        write!(f, "{}", lines.join("\n"))
    }
}

/// Splits a path into the first segment and the rest.
/// Zero allocation - returns slices into the original string.
#[inline]
fn split_path(path: &str) -> (&str, &str) {
    match path.find('/') {
        Some(idx) => (&path[..idx], &path[idx + 1..]),
        None => (path, ""),
    }
}

#[cfg(test)]
mod tests;
