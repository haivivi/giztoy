/// KV key layout for the memory package.
///
/// All keys are scoped under a memory prefix ("mem:{mid}") which is managed
/// by the recall.Index for segments and graph data. The memory package adds
/// keys for conversations.
///
/// ```text
/// {mid}:conv:{convID}:msg:{ts_ns}   → msgpack Message
/// {mid}:conv:{convID}:revert        → timestamp string (revert point)
/// ```
///
/// Segment and graph keys are managed by the recall layer:
/// ```text
/// {mid}:seg:{bucket}:{ts_ns}        → msgpack Segment
/// {mid}:sid:{id}                    → "{bucket}:{ts_ns}"
/// {mid}:g:e:{label}                 → Entity
/// {mid}:g:r:{from}:{type}:{to}     → Relation
/// ```
/// Base KV prefix for a memory ID. Format: "mem:{mid}"
pub fn mem_prefix(mid: &str) -> String {
    format!("mem:{mid}")
}

/// KV key for host-level metadata. Format: "mem:__meta:{name}"
pub fn host_meta_key(name: &str) -> String {
    format!("mem:__meta:{name}")
}

/// KV key for a conversation message.
/// Format: "mem:{mid}:conv:{convID}:msg:{ts_ns}"
/// Timestamp is zero-padded to 20 digits for correct lexicographic ordering.
pub fn conv_msg_key(mid: &str, conv_id: &str, ts: i64) -> String {
    format!("mem:{mid}:conv:{conv_id}:msg:{ts:020}")
}

/// Prefix for listing all messages in a conversation.
/// Format: "mem:{mid}:conv:{convID}:msg:"
pub fn conv_msg_prefix(mid: &str, conv_id: &str) -> String {
    format!("mem:{mid}:conv:{conv_id}:msg:")
}

/// KV key for a conversation revert point.
/// Format: "mem:{mid}:conv:{convID}:revert"
pub fn conv_revert_key(mid: &str, conv_id: &str) -> String {
    format!("mem:{mid}:conv:{conv_id}:revert")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_conv_msg_key_format() {
        let key = conv_msg_key("cat_girl", "dev1", 1700000000000000000);
        assert_eq!(key, "mem:cat_girl:conv:dev1:msg:01700000000000000000");
    }

    #[test]
    fn test_conv_msg_prefix_format() {
        let prefix = conv_msg_prefix("cat_girl", "dev1");
        assert_eq!(prefix, "mem:cat_girl:conv:dev1:msg:");
    }

    #[test]
    fn test_conv_revert_key_format() {
        let key = conv_revert_key("cat_girl", "dev1");
        assert_eq!(key, "mem:cat_girl:conv:dev1:revert");
    }

    #[test]
    fn test_conv_msg_key_lexicographic_order() {
        let k1 = conv_msg_key("m", "c", 9000);
        let k2 = conv_msg_key("m", "c", 10000);
        assert!(k1 < k2, "zero-padded timestamps must sort correctly: {k1} < {k2}");
    }
}
