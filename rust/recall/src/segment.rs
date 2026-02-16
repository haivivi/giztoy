use std::collections::HashSet;

use crate::error::RecallError;
use crate::index::RecallIndex;
use crate::keys::{segment_key, segment_prefix, sid_key};
use crate::types::{ScoredSegment, SearchQuery, Segment};

/// Score fusion weights for combining search signals.
const WEIGHT_VECTOR: f64 = 0.5;
const WEIGHT_KEYWORD: f64 = 0.3;
const WEIGHT_LABEL: f64 = 0.2;

impl RecallIndex {
    /// Persist a segment in the index.
    ///
    /// Stores segment data (msgpack), writes sid reverse index, and optionally
    /// indexes the vector if embedder + vec are configured.
    pub async fn store_segment(&self, seg: &Segment) -> Result<(), RecallError> {
        let data = rmp_serde::to_vec_named(seg)
            .map_err(|e| RecallError::Serialization(e.to_string()))?;

        let seg_k = segment_key(&self.prefix, seg.timestamp);
        let sid_k = sid_key(&self.prefix, &seg.id);
        let ts_bytes = seg.timestamp.to_string();

        // Check if this ID already exists with a different timestamp.
        let old_ts = self.lookup_segment_ts(&seg.id);

        // Write new segment + sid atomically.
        self.store.batch_set(&[
            (seg_k.as_str(), data.as_slice()),
            (sid_k.as_str(), ts_bytes.as_bytes()),
        ])?;

        // Delete old segment key if timestamp changed.
        if let Ok(ts) = old_ts {
            if ts != seg.timestamp {
                let old_key = segment_key(&self.prefix, ts);
                let _ = self.store.delete(&old_key);
            }
        }

        // Index vector if both embedder and vec are available.
        if let (Some(embedder), Some(vec)) = (&self.embedder, &self.vec) {
            let embedding = embedder.embed(&seg.summary).await?;
            vec.insert(&seg.id, &embedding)?;
        }

        Ok(())
    }

    /// Remove a segment by ID.
    pub fn delete_segment(&self, id: &str) -> Result<(), RecallError> {
        let ts = match self.lookup_segment_ts(id) {
            Ok(ts) => ts,
            Err(_) => return Ok(()), // not found is not an error
        };

        let seg_k = segment_key(&self.prefix, ts);
        let sid_k = sid_key(&self.prefix, id);
        self.store.batch_delete(&[seg_k.as_str(), sid_k.as_str()])?;

        if let Some(vec) = &self.vec {
            let _ = vec.delete(id);
        }
        Ok(())
    }

    /// Retrieve a segment by ID. Returns None if not found.
    pub fn get_segment(&self, id: &str) -> Result<Option<Segment>, RecallError> {
        let ts = match self.lookup_segment_ts(id) {
            Ok(ts) => ts,
            Err(_) => return Ok(None),
        };

        let data = match self.store.get(&segment_key(&self.prefix, ts))? {
            Some(data) => data,
            None => return Ok(None),
        };

        let seg: Segment = rmp_serde::from_slice(&data)
            .map_err(|e| RecallError::Serialization(e.to_string()))?;
        Ok(Some(seg))
    }

    /// Return the n most recent segments, newest first.
    pub fn recent_segments(&self, n: usize) -> Result<Vec<Segment>, RecallError> {
        if n == 0 {
            return Ok(vec![]);
        }

        let prefix = segment_prefix(&self.prefix);
        let entries = self.store.scan(&prefix)?;

        let mut all = Vec::new();
        for (_key, value) in entries {
            if let Ok(seg) = rmp_serde::from_slice::<Segment>(&value) {
                all.push(seg);
            }
        }

        // KV scan is ascending; reverse for newest first.
        all.reverse();
        all.truncate(n);
        Ok(all)
    }

    /// Search segments using multi-signal scoring.
    ///
    /// Score = 0.5 * vector_similarity + 0.3 * keyword_overlap + 0.2 * label_overlap
    pub async fn search_segments(
        &self,
        q: &SearchQuery,
    ) -> Result<Vec<ScoredSegment>, RecallError> {
        let limit = if q.limit == 0 { 10 } else { q.limit };

        // Load all segments with time + label filtering.
        let segments = self.load_segments(q)?;
        if segments.is_empty() {
            return Ok(vec![]);
        }

        // Vector scores.
        let mut vec_scores = std::collections::HashMap::new();
        if let (Some(embedder), Some(vec)) = (&self.embedder, &self.vec) {
            if !q.text.is_empty() && vec.len() > 0 {
                let query_vec = embedder.embed(&q.text).await?;
                let top_k = (limit * 3).max(20);
                let matches = vec.search(&query_vec, top_k)?;
                for m in matches {
                    let sim = (1.0 - m.distance as f64 / 2.0).max(0.0);
                    vec_scores.insert(m.id, sim);
                }
            }
        }

        // Keyword and label preparation.
        let query_terms = tokenize(&q.text);
        let label_set: HashSet<&str> = q.labels.iter().map(|s| s.as_str()).collect();
        let has_vec = !vec_scores.is_empty();
        let has_keywords = !query_terms.is_empty();
        let has_labels = !label_set.is_empty();

        // Score each segment.
        let mut scored = Vec::new();
        for seg in segments {
            let mut score = 0.0;

            if has_vec {
                if let Some(&vs) = vec_scores.get(&seg.id) {
                    score += WEIGHT_VECTOR * vs;
                }
            }

            if has_keywords {
                score += WEIGHT_KEYWORD * keyword_score(&query_terms, &seg.keywords);
            }

            if has_labels {
                score += WEIGHT_LABEL * label_score(&seg.labels, &label_set);
            }

            if score == 0.0 && (has_vec || has_keywords || has_labels) {
                continue;
            }

            scored.push(ScoredSegment {
                segment: seg,
                score,
            });
        }

        // Sort by score desc, then timestamp desc.
        scored.sort_by(|a, b| {
            b.score
                .partial_cmp(&a.score)
                .unwrap_or(std::cmp::Ordering::Equal)
                .then_with(|| b.segment.timestamp.cmp(&a.segment.timestamp))
        });

        scored.truncate(limit);
        Ok(scored)
    }

    fn load_segments(&self, q: &SearchQuery) -> Result<Vec<Segment>, RecallError> {
        let prefix = segment_prefix(&self.prefix);
        let entries = self.store.scan(&prefix)?;

        let after_ns = q.after_ns;
        let before_ns = if q.before_ns == 0 { i64::MAX } else { q.before_ns };
        let label_set: HashSet<&str> = q.labels.iter().map(|s| s.as_str()).collect();
        let filter_labels = !label_set.is_empty();

        let mut segments = Vec::new();
        for (_key, value) in entries {
            let seg: Segment = match rmp_serde::from_slice(&value) {
                Ok(s) => s,
                Err(_) => continue,
            };

            if seg.timestamp < after_ns || seg.timestamp >= before_ns {
                continue;
            }

            if filter_labels && !seg.labels.iter().any(|l| label_set.contains(l.as_str())) {
                continue;
            }

            segments.push(seg);
        }
        Ok(segments)
    }

    fn lookup_segment_ts(&self, id: &str) -> Result<i64, RecallError> {
        let key = sid_key(&self.prefix, id);
        match self.store.get(&key)? {
            Some(data) => {
                let s = String::from_utf8(data)
                    .map_err(|e| RecallError::Serialization(e.to_string()))?;
                s.parse::<i64>()
                    .map_err(|e| RecallError::Serialization(e.to_string()))
            }
            None => Err(RecallError::Storage("not found".into())),
        }
    }
}

fn keyword_score(query_terms: &[String], seg_keywords: &[String]) -> f64 {
    if query_terms.is_empty() {
        return 0.0;
    }
    let seg_set: HashSet<String> = seg_keywords.iter().map(|kw| kw.to_lowercase()).collect();
    let hits = query_terms.iter().filter(|qt| seg_set.contains(*qt)).count();
    hits as f64 / query_terms.len() as f64
}

fn label_score(seg_labels: &[String], query_label_set: &HashSet<&str>) -> f64 {
    if seg_labels.is_empty() {
        return 0.0;
    }
    let hits = seg_labels
        .iter()
        .filter(|l| query_label_set.contains(l.as_str()))
        .count();
    hits as f64 / seg_labels.len() as f64
}

/// Tokenize: lowercase + split whitespace + dedup.
fn tokenize(text: &str) -> Vec<String> {
    if text.is_empty() {
        return vec![];
    }
    let mut seen = HashSet::new();
    text.to_lowercase()
        .split_whitespace()
        .filter(|w| seen.insert(w.to_string()))
        .map(|w| w.to_string())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tokenize() {
        assert_eq!(tokenize(""), Vec::<String>::new());
        assert_eq!(tokenize("Hello World"), vec!["hello", "world"]);
        assert_eq!(tokenize("hello hello world"), vec!["hello", "world"]);
        assert_eq!(tokenize("  A  B  "), vec!["a", "b"]);
    }

    #[test]
    fn test_keyword_score_fn() {
        let query = vec!["hello".to_string(), "world".to_string()];
        let kw = vec!["Hello".to_string(), "Foo".to_string()];
        let score = keyword_score(&query, &kw);
        assert!((score - 0.5).abs() < 0.001);
    }

    #[test]
    fn test_label_score_fn() {
        let labels = vec!["a".to_string(), "b".to_string(), "c".to_string()];
        let query_set: HashSet<&str> = ["a", "c"].into_iter().collect();
        let score = label_score(&labels, &query_set);
        assert!((score - 2.0 / 3.0).abs() < 0.001);
    }
}
