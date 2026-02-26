use std::collections::HashSet;

use crate::error::RecallError;
use crate::index::RecallIndex;
use crate::keys::{bucket_prefix, parse_sid_value, segment_key, segment_prefix, sid_key, sid_value};
use crate::types::{Bucket, ScoredSegment, SearchQuery, Segment};

/// Score fusion weights for combining search signals.
const WEIGHT_VECTOR: f64 = 0.5;
const WEIGHT_KEYWORD: f64 = 0.3;
const WEIGHT_LABEL: f64 = 0.2;

impl RecallIndex {
    /// Persist a segment in the index.
    ///
    /// The segment's bucket field determines which bucket it is stored in.
    /// If bucket is empty, it defaults to "1h".
    pub async fn store_segment(&self, seg: &Segment) -> Result<(), RecallError> {
        let bucket = if seg.bucket.is_empty() { Bucket::BUCKET_1H } else { &seg.bucket };

        let data = rmp_serde::to_vec_named(seg)
            .map_err(|e| RecallError::Serialization(e.to_string()))?;

        let seg_k = segment_key(&self.prefix, bucket, seg.timestamp);
        let sid_k = sid_key(&self.prefix, &seg.id);
        let sid_v = sid_value(bucket, seg.timestamp);

        let old_loc = self.lookup_segment_loc(&seg.id);

        self.store.batch_set(&[
            (seg_k.as_str(), data.as_slice()),
            (sid_k.as_str(), sid_v.as_bytes()),
        ])?;

        if let Ok((old_bucket, old_ts)) = old_loc {
            if old_ts != seg.timestamp || old_bucket != bucket {
                let old_key = segment_key(&self.prefix, &old_bucket, old_ts);
                let _ = self.store.delete(&old_key);
            }
        }

        if let (Some(embedder), Some(vec)) = (&self.embedder, &self.vec) {
            let embedding = embedder.embed(&seg.summary).await?;
            vec.insert(&seg.id, &embedding)?;
        }

        Ok(())
    }

    /// Remove a segment by ID.
    pub fn delete_segment(&self, id: &str) -> Result<(), RecallError> {
        let (bucket, ts) = match self.lookup_segment_loc(id) {
            Ok(loc) => loc,
            Err(_) => return Ok(()),
        };

        let seg_k = segment_key(&self.prefix, &bucket, ts);
        let sid_k = sid_key(&self.prefix, id);
        self.store.batch_delete(&[seg_k.as_str(), sid_k.as_str()])?;

        if let Some(vec) = &self.vec {
            let _ = vec.delete(id);
        }
        Ok(())
    }

    /// Retrieve a segment by ID. Returns None if not found.
    pub fn get_segment(&self, id: &str) -> Result<Option<Segment>, RecallError> {
        let (bucket, ts) = match self.lookup_segment_loc(id) {
            Ok(loc) => loc,
            Err(_) => return Ok(None),
        };

        let data = match self.store.get(&segment_key(&self.prefix, &bucket, ts))? {
            Some(data) => data,
            None => return Ok(None),
        };

        let seg: Segment = rmp_serde::from_slice(&data)
            .map_err(|e| RecallError::Serialization(e.to_string()))?;
        Ok(Some(seg))
    }

    /// Return the n most recent segments across all buckets, newest first.
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

        all.sort_by(|a, b| b.timestamp.cmp(&a.timestamp));
        all.truncate(n);
        Ok(all)
    }

    /// Return all segments in a specific bucket in KV scan order.
    ///
    /// The underlying KV contract guarantees scan returns keys in sorted order.
    /// With segment keys encoded as `{prefix}:seg:{bucket}:{ts_20d}`, this is the
    /// effective ordering used by compaction.
    pub fn bucket_segments(&self, bucket: &Bucket) -> Result<Vec<Segment>, RecallError> {
        let prefix = bucket_prefix(&self.prefix, bucket.as_str());
        let entries = self.store.scan(&prefix)?;

        let mut segments = Vec::new();
        for (_key, value) in entries {
            if let Ok(seg) = rmp_serde::from_slice::<Segment>(&value) {
                segments.push(seg);
            }
        }
        Ok(segments)
    }

    /// Return segment count and total summary char count in a bucket.
    pub fn bucket_stats(&self, bucket: &Bucket) -> Result<(usize, usize), RecallError> {
        let prefix = bucket_prefix(&self.prefix, bucket.as_str());
        let entries = self.store.scan(&prefix)?;

        let mut count = 0;
        let mut chars = 0;
        for (_key, value) in entries {
            if let Ok(seg) = rmp_serde::from_slice::<Segment>(&value) {
                count += 1;
                chars += seg.summary.len();
            }
        }
        Ok((count, chars))
    }

    /// Search segments using multi-signal scoring.
    pub async fn search_segments(
        &self,
        q: &SearchQuery,
    ) -> Result<Vec<ScoredSegment>, RecallError> {
        let limit = if q.limit == 0 { 10 } else { q.limit };

        let segments = self.load_segments(q)?;
        if segments.is_empty() {
            return Ok(vec![]);
        }

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

        let query_terms = tokenize(&q.text);
        let label_set: HashSet<&str> = q.labels.iter().map(|s| s.as_str()).collect();
        let has_vec = !vec_scores.is_empty();
        let has_keywords = !query_terms.is_empty();
        let has_labels = !label_set.is_empty();

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

    fn lookup_segment_loc(&self, id: &str) -> Result<(String, i64), RecallError> {
        let key = sid_key(&self.prefix, id);
        match self.store.get(&key)? {
            Some(data) => {
                parse_sid_value(&data).map_err(|e| RecallError::Serialization(e))
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
