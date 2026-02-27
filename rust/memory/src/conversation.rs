use giztoy_recall::Segment;

use crate::error::MemoryError;
use crate::keys::{conv_msg_key, conv_msg_prefix, conv_revert_key};
use crate::memory::Memory;
use crate::types::{Message, Role, now_nano};

/// Active dialogue session tied to a device or session ID.
/// Messages are stored in KV keyed by nanosecond timestamp for ordering.
pub struct Conversation<'m> {
    mem: &'m Memory,
    conv_id: String,
    labels: Vec<String>,

    pending_chars: usize,
    pending_msgs: usize,
    last_compress_err: Option<MemoryError>,
}

impl<'m> Conversation<'m> {
    pub(crate) fn new(mem: &'m Memory, conv_id: String, labels: Vec<String>) -> Self {
        Self {
            mem,
            conv_id,
            labels,
            pending_chars: 0,
            pending_msgs: 0,
            last_compress_err: None,
        }
    }

    pub fn id(&self) -> &str { &self.conv_id }

    pub fn labels(&self) -> &[String] { &self.labels }

    pub fn last_compress_err(&self) -> Option<&MemoryError> {
        self.last_compress_err.as_ref()
    }

    /// Store a message. Auto-fills timestamp if zero.
    /// Saves revert point on user messages.
    /// Triggers auto-compression when policy thresholds are reached.
    pub async fn append(&mut self, mut msg: Message) -> Result<(), MemoryError> {
        if msg.timestamp == 0 {
            msg.timestamp = now_nano();
        }

        let data = rmp_serde::to_vec_named(&msg)
            .map_err(|e| MemoryError::Serialization(e.to_string()))?;

        let store = self.mem.kv_store();
        let mid = self.mem.id();

        let key = conv_msg_key(mid, &self.conv_id, msg.timestamp);
        store.set(&key, &data)?;

        if msg.role == Role::User {
            let rk = conv_revert_key(mid, &self.conv_id);
            let ts_str = msg.timestamp.to_string();
            store.set(&rk, ts_str.as_bytes())?;
        }

        self.pending_chars += msg.content.len();
        self.pending_msgs += 1;

        if self.mem.has_compressor()
            && self.mem.policy().should_compress(self.pending_chars, self.pending_msgs)
        {
            match self.mem.compress(self, None).await {
                Ok(()) => {
                    self.pending_chars = 0;
                    self.pending_msgs = 0;
                    self.last_compress_err = None;

                    if let Err(e) = self.mem.compact().await {
                        self.last_compress_err = Some(e);
                    }
                }
                Err(e) => {
                    self.last_compress_err = Some(e);
                }
            }
        }

        Ok(())
    }

    /// Return the n most recent messages in chronological order (oldest first).
    pub fn recent(&self, n: usize) -> Result<Vec<Message>, MemoryError> {
        if n == 0 {
            return Ok(vec![]);
        }

        let all = self.scan_messages()?;
        let start = if all.len() > n { all.len() - n } else { 0 };
        Ok(all[start..].to_vec())
    }

    /// Return total message count.
    pub fn count(&self) -> Result<usize, MemoryError> {
        let prefix = conv_msg_prefix(self.mem.id(), &self.conv_id);
        let entries = self.mem.kv_store().scan(&prefix)?;
        Ok(entries.len())
    }

    /// Revert: delete the last user message and all subsequent model replies.
    pub fn revert(&self) -> Result<(), MemoryError> {
        let store = self.mem.kv_store();
        let mid = self.mem.id();

        let rk = conv_revert_key(mid, &self.conv_id);
        let data = match store.get(&rk)? {
            Some(d) => d,
            None => return Ok(()),
        };

        let revert_ts: i64 = String::from_utf8(data)
            .map_err(|e| MemoryError::Serialization(e.to_string()))?
            .parse()
            .map_err(|e: std::num::ParseIntError| MemoryError::Serialization(e.to_string()))?;

        let prefix = conv_msg_prefix(mid, &self.conv_id);
        let entries = store.scan(&prefix)?;

        let mut to_delete: Vec<String> = Vec::new();
        for (key, _) in &entries {
            let ts_str = match key.rsplit(':').next() {
                Some(s) => s,
                None => continue,
            };
            let ts: i64 = match ts_str.parse() {
                Ok(t) => t,
                Err(_) => continue,
            };
            if ts >= revert_ts {
                to_delete.push(key.clone());
            }
        }

        if to_delete.is_empty() {
            return Ok(());
        }

        let key_refs: Vec<&str> = to_delete.iter().map(|s| s.as_str()).collect();
        store.batch_delete(&key_refs)?;

        // Find the new latest user message for the updated revert point.
        let remaining = store.scan(&prefix)?;
        let mut latest_user_ts: i64 = 0;
        for (_, value) in &remaining {
            if let Ok(msg) = rmp_serde::from_slice::<Message>(value)
                && msg.role == Role::User
                && msg.timestamp > latest_user_ts
            {
                latest_user_ts = msg.timestamp;
            }
        }

        if latest_user_ts > 0 {
            let ts_str = latest_user_ts.to_string();
            store.set(&rk, ts_str.as_bytes())?;
        } else {
            store.delete(&rk)?;
        }

        Ok(())
    }

    /// Return the n most recent memory segments from the recall index.
    pub fn recent_segments(&self, n: usize) -> Result<Vec<Segment>, MemoryError> {
        Ok(self.mem.index().recent_segments(n)?)
    }

    /// Return all messages in chronological order.
    pub fn all(&self) -> Result<Vec<Message>, MemoryError> {
        self.scan_messages()
    }

    /// Remove all messages and the revert point. Resets pending compression
    /// counters and last compression error in this conversation handle.
    pub fn clear(&mut self) -> Result<(), MemoryError> {
        let store = self.mem.kv_store();
        let mid = self.mem.id();
        let prefix = conv_msg_prefix(mid, &self.conv_id);

        let entries = store.scan(&prefix)?;
        let mut keys: Vec<String> = entries.into_iter().map(|(k, _)| k).collect();
        keys.push(conv_revert_key(mid, &self.conv_id));

        if !keys.is_empty() {
            let key_refs: Vec<&str> = keys.iter().map(|s| s.as_str()).collect();
            store.batch_delete(&key_refs)?;
        }

        self.pending_chars = 0;
        self.pending_msgs = 0;
        self.last_compress_err = None;

        Ok(())
    }

    fn scan_messages(&self) -> Result<Vec<Message>, MemoryError> {
        let prefix = conv_msg_prefix(self.mem.id(), &self.conv_id);
        let entries = self.mem.kv_store().scan(&prefix)?;

        let mut msgs = Vec::new();
        for (_, value) in entries {
            match rmp_serde::from_slice::<Message>(&value) {
                Ok(msg) => msgs.push(msg),
                Err(_) => continue,
            }
        }
        Ok(msgs)
    }
}
