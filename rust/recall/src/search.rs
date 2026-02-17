use giztoy_graph::Graph;

use crate::error::RecallError;
use crate::index::RecallIndex;
use crate::types::{Query, SearchQuery, SearchResult};

impl RecallIndex {
    /// Combined search: graph expansion followed by segment search.
    ///
    /// 1. Expand seed labels through the graph (BFS, up to Hops).
    /// 2. Build a SearchQuery with expanded labels and query text.
    /// 3. Run search_segments to find matching segments.
    pub async fn search(&self, q: &Query) -> Result<SearchResult, RecallError> {
        let hops = if q.hops == 0 { 2 } else { q.hops };
        let limit = if q.limit == 0 { 10 } else { q.limit };

        // Step 1: Graph expansion.
        let expanded = if !q.labels.is_empty() {
            let seeds: Vec<&str> = q.labels.iter().map(|s| s.as_str()).collect();
            self.graph.expand(&seeds, hops)?
        } else {
            vec![]
        };

        // Step 2: Build segment search query.
        let sq = SearchQuery {
            text: q.text.clone(),
            labels: expanded.clone(),
            limit,
            after_ns: 0,
            before_ns: 0,
        };

        // Step 3: Search segments.
        let segments = self.search_segments(&sq).await?;

        Ok(SearchResult {
            segments,
            expanded,
        })
    }
}
