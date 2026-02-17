use std::collections::{BinaryHeap, HashMap, HashSet};
use std::sync::RwLock;

use crate::cosine::cosine_distance;
use crate::error::VecError;
use crate::vecstore::{Match, VecIndex};

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

/// HNSWConfig configures a new HNSW index.
#[derive(Debug, Clone)]
pub struct HNSWConfig {
    /// Vector dimension. Required; must be positive.
    pub dim: usize,
    /// Max connections per node per layer (except layer 0 which allows 2*M).
    /// Default: 16.
    pub m: usize,
    /// Size of the dynamic candidate list during index building.
    /// Default: 200.
    pub ef_construction: usize,
    /// Default size of the dynamic candidate list during search.
    /// Default: 50.
    pub ef_search: usize,
}

impl HNSWConfig {
    pub(crate) fn set_defaults(&mut self) {
        if self.m < 2 {
            self.m = 16;
        }
        if self.ef_construction == 0 {
            self.ef_construction = 200;
        }
        if self.ef_search == 0 {
            self.ef_search = 50;
        }
    }

    fn max_conns(&self, layer: usize) -> usize {
        if layer == 0 {
            self.m * 2
        } else {
            self.m
        }
    }
}

// ---------------------------------------------------------------------------
// Internal priority-queue types
// ---------------------------------------------------------------------------

#[derive(Clone)]
struct DistItem {
    id: u32,
    dist: f32,
}

/// Min-heap: closest first.
impl Ord for DistItem {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        other
            .dist
            .partial_cmp(&self.dist)
            .unwrap_or(std::cmp::Ordering::Equal)
    }
}
impl PartialOrd for DistItem {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}
impl PartialEq for DistItem {
    fn eq(&self, other: &Self) -> bool {
        self.dist == other.dist && self.id == other.id
    }
}
impl Eq for DistItem {}

/// Reversed for max-heap usage: farthest first.
#[derive(Clone)]
struct MaxDistItem {
    id: u32,
    dist: f32,
}

impl Ord for MaxDistItem {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        self.dist
            .partial_cmp(&other.dist)
            .unwrap_or(std::cmp::Ordering::Equal)
    }
}
impl PartialOrd for MaxDistItem {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}
impl PartialEq for MaxDistItem {
    fn eq(&self, other: &Self) -> bool {
        self.dist == other.dist && self.id == other.id
    }
}
impl Eq for MaxDistItem {}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

pub(crate) struct HnswNode {
    pub(crate) id: String,
    pub(crate) vector: Vec<f32>,
    pub(crate) level: usize,
    pub(crate) friends: Vec<Vec<u32>>, // friends[layer] = neighbor internal IDs
}

// ---------------------------------------------------------------------------
// HNSW
// ---------------------------------------------------------------------------

pub(crate) struct HnswInner {
    pub(crate) cfg: HNSWConfig,
    pub(crate) nodes: Vec<Option<HnswNode>>,
    pub(crate) id_map: HashMap<String, u32>,
    pub(crate) entry_id: i32,
    pub(crate) max_level: usize,
    pub(crate) count: usize,
    pub(crate) free: Vec<u32>,
    pub(crate) level_mul: f64,
}

/// HNSW is a Hierarchical Navigable Small World index implementing [VecIndex].
///
/// All methods are safe for concurrent use (via RwLock).
pub struct HNSW {
    inner: RwLock<HnswInner>,
}

impl HNSW {
    /// Create an empty HNSW index with the given configuration.
    /// Panics if `cfg.dim` is not positive.
    pub fn new(mut cfg: HNSWConfig) -> Self {
        assert!(cfg.dim > 0, "vecstore: HNSWConfig.dim must be positive");
        cfg.set_defaults();
        let level_mul = 1.0 / (cfg.m as f64).ln();
        Self {
            inner: RwLock::new(HnswInner {
                cfg,
                nodes: Vec::new(),
                id_map: HashMap::new(),
                entry_id: -1,
                max_level: 0,
                count: 0,
                free: Vec::new(),
                level_mul,
            }),
        }
    }

    /// Adjust the search-time candidate list size.
    pub fn set_ef_search(&self, ef: usize) {
        self.inner.write().unwrap().cfg.ef_search = ef;
    }

    /// Build from deserialized state (used by LoadHNSW).
    pub(crate) fn from_inner(inner: HnswInner) -> Self {
        Self {
            inner: RwLock::new(inner),
        }
    }

    /// Access inner for serialization (used by Save).
    pub(crate) fn read_inner(&self) -> std::sync::RwLockReadGuard<'_, HnswInner> {
        self.inner.read().unwrap()
    }
}

impl HnswInner {
    fn random_level(&self) -> usize {
        let mut rng = rand::thread_rng();
        let r: f64 = rand::Rng::r#gen::<f64>(&mut rng).max(f64::MIN_POSITIVE);
        let level = (-r.ln() * self.level_mul) as usize;
        level.min(31)
    }

    fn search_layer(
        &self,
        query: &[f32],
        entry_points: &[u32],
        ef: usize,
        layer: usize,
    ) -> Vec<u32> {
        let mut visited = HashSet::with_capacity(ef * 2);
        let mut candidates: BinaryHeap<DistItem> = BinaryHeap::new();
        let mut results: BinaryHeap<MaxDistItem> = BinaryHeap::new();

        for &ep in entry_points {
            if let Some(nd) = &self.nodes[ep as usize] {
                visited.insert(ep);
                let d = cosine_distance(query, &nd.vector);
                candidates.push(DistItem { id: ep, dist: d });
                results.push(MaxDistItem { id: ep, dist: d });
            }
        }

        while let Some(closest) = candidates.pop() {
            if results.len() >= ef {
                if let Some(farthest) = results.peek() {
                    if closest.dist > farthest.dist {
                        break;
                    }
                }
            }

            if let Some(nd) = &self.nodes[closest.id as usize] {
                if layer < nd.friends.len() {
                    for &f_id in &nd.friends[layer] {
                        if visited.contains(&f_id) {
                            continue;
                        }
                        visited.insert(f_id);

                        if let Some(fn_node) = &self.nodes[f_id as usize] {
                            let d = cosine_distance(query, &fn_node.vector);
                            let should_add = results.len() < ef
                                || results
                                    .peek()
                                    .map_or(true, |far| d < far.dist);
                            if should_add {
                                candidates.push(DistItem { id: f_id, dist: d });
                                results.push(MaxDistItem { id: f_id, dist: d });
                                if results.len() > ef {
                                    results.pop();
                                }
                            }
                        }
                    }
                }
            }
        }

        results.into_iter().map(|item| item.id).collect()
    }

    fn select_closest(&self, query: &[f32], candidates: &[u32], max_n: usize) -> Vec<u32> {
        if candidates.len() <= max_n {
            return candidates.to_vec();
        }

        let mut items: Vec<(u32, f32)> = candidates
            .iter()
            .filter_map(|&c_id| {
                self.nodes[c_id as usize]
                    .as_ref()
                    .map(|nd| (c_id, cosine_distance(query, &nd.vector)))
            })
            .collect();

        items.sort_by(|a, b| a.1.partial_cmp(&b.1).unwrap_or(std::cmp::Ordering::Equal));
        if items.len() > max_n {
            items.truncate(max_n);
        }
        items.into_iter().map(|(id, _)| id).collect()
    }

    fn remove_locked(&mut self, idx: u32) {
        let nd = match self.nodes[idx as usize].take() {
            Some(nd) => nd,
            None => return,
        };

        // Disconnect from all neighbors at every layer.
        for lev in 0..=nd.level {
            if lev < nd.friends.len() {
                for &f_id in &nd.friends[lev] {
                    if let Some(fn_node) = &mut self.nodes[f_id as usize] {
                        if lev < fn_node.friends.len() {
                            fn_node.friends[lev].retain(|&x| x != idx);
                        }
                    }
                }
            }
        }

        self.id_map.remove(&nd.id);
        self.free.push(idx);
        self.count -= 1;

        if self.entry_id == idx as i32 {
            self.find_new_entry();
        }
    }

    fn find_new_entry(&mut self) {
        if self.count == 0 {
            self.entry_id = -1;
            self.max_level = 0;
            return;
        }
        let mut best: i32 = -1;
        let mut best_level: i32 = -1;
        for (i, nd) in self.nodes.iter().enumerate() {
            if let Some(nd) = nd {
                if nd.level as i32 > best_level {
                    best = i as i32;
                    best_level = nd.level as i32;
                }
            }
        }
        if best < 0 {
            self.entry_id = -1;
            self.max_level = 0;
            self.count = 0;
            return;
        }
        self.entry_id = best;
        self.max_level = best_level as usize;
    }
}

impl VecIndex for HNSW {
    fn insert(&self, id: &str, vector: &[f32]) -> Result<(), VecError> {
        let mut inner = self.inner.write().unwrap();
        if vector.len() != inner.cfg.dim {
            return Err(VecError::DimensionMismatch {
                got: vector.len(),
                want: inner.cfg.dim,
            });
        }

        let vec = vector.to_vec();

        // Replace existing entry if present.
        if let Some(&old_idx) = inner.id_map.get(id) {
            inner.remove_locked(old_idx);
        }

        // Allocate internal ID.
        let idx = if let Some(free_idx) = inner.free.pop() {
            free_idx
        } else {
            let idx = inner.nodes.len() as u32;
            inner.nodes.push(None);
            idx
        };

        let level = inner.random_level();
        let nd = HnswNode {
            id: id.to_string(),
            vector: vec.clone(),
            level,
            friends: vec![Vec::new(); level + 1],
        };
        inner.nodes[idx as usize] = Some(nd);
        inner.id_map.insert(id.to_string(), idx);
        inner.count += 1;

        // First node — set as entry point.
        if inner.entry_id < 0 {
            inner.entry_id = idx as i32;
            inner.max_level = level;
            return Ok(());
        }

        // Phase 1: Greedy descent from top layer to level+1.
        let mut cur = inner.entry_id as u32;
        let mut cur_dist = cosine_distance(&vec, &inner.nodes[cur as usize].as_ref().unwrap().vector);

        let top = inner.max_level;
        for lev in (level + 1..=top).rev() {
            let mut changed = true;
            while changed {
                changed = false;
                if let Some(cur_node) = &inner.nodes[cur as usize] {
                    if lev < cur_node.friends.len() {
                        for &f_id in &cur_node.friends[lev] {
                            if let Some(fn_node) = &inner.nodes[f_id as usize] {
                                let d = cosine_distance(&vec, &fn_node.vector);
                                if d < cur_dist {
                                    cur = f_id;
                                    cur_dist = d;
                                    changed = true;
                                }
                            }
                        }
                    }
                } else {
                    break;
                }
            }
        }

        // Phase 2: Beam search + connect at each layer.
        let top_insert = level.min(inner.max_level);
        let ef_construction = inner.cfg.ef_construction;

        let mut ep = vec![cur];
        for lev in (0..=top_insert).rev() {
            let candidates = inner.search_layer(&vec, &ep, ef_construction, lev);
            let max_c = inner.cfg.max_conns(lev);
            let neighbors = inner.select_closest(&vec, &candidates, max_c);

            // Set friends for new node.
            if let Some(nd) = &mut inner.nodes[idx as usize] {
                nd.friends[lev] = neighbors.clone();
            }

            // Bidirectional connections + pruning.
            for &n_id in &neighbors {
                // First, add the connection.
                let needs_prune = if let Some(nn) = &mut inner.nodes[n_id as usize] {
                    if lev < nn.friends.len() {
                        nn.friends[lev].push(idx);
                        nn.friends[lev].len() > max_c
                    } else {
                        false
                    }
                } else {
                    false
                };
                // Prune in a separate scope to avoid simultaneous borrows.
                if needs_prune {
                    if let Some(nn) = &inner.nodes[n_id as usize] {
                        let nn_vec = nn.vector.clone();
                        let nn_friends = nn.friends[lev].clone();
                        let pruned = inner.select_closest(&nn_vec, &nn_friends, max_c);
                        if let Some(nn) = &mut inner.nodes[n_id as usize] {
                            nn.friends[lev] = pruned;
                        }
                    }
                }
            }

            ep = candidates;
        }

        // Update entry point if new node is higher.
        if level > inner.max_level {
            inner.entry_id = idx as i32;
            inner.max_level = level;
        }

        Ok(())
    }

    fn batch_insert(&self, ids: &[&str], vectors: &[&[f32]]) -> Result<(), VecError> {
        if ids.len() != vectors.len() {
            return Err(VecError::BatchLengthMismatch {
                ids: ids.len(),
                vectors: vectors.len(),
            });
        }
        for (id, vec) in ids.iter().zip(vectors.iter()) {
            self.insert(id, vec)?;
        }
        Ok(())
    }

    fn search(&self, query: &[f32], top_k: usize) -> Result<Vec<Match>, VecError> {
        let inner = self.inner.read().unwrap();
        if query.len() != inner.cfg.dim {
            return Err(VecError::DimensionMismatch {
                got: query.len(),
                want: inner.cfg.dim,
            });
        }
        if inner.count == 0 || top_k == 0 {
            return Ok(vec![]);
        }

        let ef = inner.cfg.ef_search.max(top_k);

        // Phase 1: Greedy descent from top layer to layer 1.
        let mut cur = inner.entry_id as u32;
        if inner.nodes[cur as usize].is_none() {
            return Ok(vec![]);
        }
        let mut cur_dist =
            cosine_distance(query, &inner.nodes[cur as usize].as_ref().unwrap().vector);

        for lev in (1..=inner.max_level).rev() {
            let mut changed = true;
            while changed {
                changed = false;
                if let Some(nd) = &inner.nodes[cur as usize] {
                    if lev < nd.friends.len() {
                        for &f_id in &nd.friends[lev] {
                            if let Some(fn_node) = &inner.nodes[f_id as usize] {
                                let d = cosine_distance(query, &fn_node.vector);
                                if d < cur_dist {
                                    cur = f_id;
                                    cur_dist = d;
                                    changed = true;
                                }
                            }
                        }
                    }
                } else {
                    break;
                }
            }
        }

        // Phase 2: Beam search at layer 0.
        let candidate_ids = inner.search_layer(query, &[cur], ef, 0);

        let mut results: Vec<(String, f32)> = candidate_ids
            .iter()
            .filter_map(|&c_id| {
                inner.nodes[c_id as usize].as_ref().map(|nd| {
                    (nd.id.clone(), cosine_distance(query, &nd.vector))
                })
            })
            .collect();

        results.sort_by(|a, b| a.1.partial_cmp(&b.1).unwrap_or(std::cmp::Ordering::Equal));
        if results.len() > top_k {
            results.truncate(top_k);
        }

        Ok(results
            .into_iter()
            .map(|(id, distance)| Match { id, distance })
            .collect())
    }

    fn delete(&self, id: &str) -> Result<(), VecError> {
        let mut inner = self.inner.write().unwrap();
        if let Some(&idx) = inner.id_map.get(id) {
            inner.remove_locked(idx);
        }
        Ok(())
    }

    fn len(&self) -> usize {
        self.inner.read().unwrap().count
    }

    fn flush(&self) -> Result<(), VecError> {
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn new_test_hnsw(dim: usize) -> HNSW {
        HNSW::new(HNSWConfig {
            dim,
            m: 8,
            ef_construction: 64,
            ef_search: 32,
        })
    }

    #[test]
    fn test_insert_and_search() {
        let h = new_test_hnsw(4);
        h.insert("a", &[1.0, 0.0, 0.0, 0.0]).unwrap();
        h.insert("b", &[0.0, 1.0, 0.0, 0.0]).unwrap();
        h.insert("c", &[0.9, 0.1, 0.0, 0.0]).unwrap();

        let matches = h.search(&[1.0, 0.0, 0.0, 0.0], 2).unwrap();
        assert_eq!(matches.len(), 2);
        assert_eq!(matches[0].id, "a");
        assert_eq!(matches[1].id, "c");
    }

    #[test]
    fn test_batch_insert() {
        let h = new_test_hnsw(3);
        h.batch_insert(
            &["a", "b", "c"],
            &[&[1.0, 0.0, 0.0], &[0.0, 1.0, 0.0], &[0.0, 0.0, 1.0]],
        )
        .unwrap();
        assert_eq!(h.len(), 3);

        let matches = h.search(&[1.0, 0.0, 0.0], 1).unwrap();
        assert_eq!(matches[0].id, "a");
    }

    #[test]
    fn test_batch_insert_mismatch() {
        let h = new_test_hnsw(3);
        assert!(h
            .batch_insert(&["a", "b"], &[&[1.0, 0.0, 0.0]])
            .is_err());
    }

    #[test]
    fn test_dimension_mismatch() {
        let h = new_test_hnsw(4);
        assert!(h.insert("a", &[1.0, 0.0, 0.0]).is_err());
        h.insert("b", &[1.0, 0.0, 0.0, 0.0]).unwrap();
        assert!(h.search(&[1.0, 0.0], 1).is_err());
    }

    #[test]
    fn test_delete() {
        let h = new_test_hnsw(3);
        h.insert("a", &[1.0, 0.0, 0.0]).unwrap();
        h.insert("b", &[0.0, 1.0, 0.0]).unwrap();
        h.insert("c", &[0.0, 0.0, 1.0]).unwrap();
        assert_eq!(h.len(), 3);

        h.delete("b").unwrap();
        assert_eq!(h.len(), 2);

        let matches = h.search(&[0.0, 1.0, 0.0], 3).unwrap();
        for m in &matches {
            assert_ne!(m.id, "b");
        }

        h.delete("nonexistent").unwrap();
    }

    #[test]
    fn test_delete_entry_point() {
        let h = new_test_hnsw(3);
        h.insert("a", &[1.0, 0.0, 0.0]).unwrap();
        h.insert("b", &[0.0, 1.0, 0.0]).unwrap();

        h.delete("a").unwrap();
        h.delete("b").unwrap();
        assert_eq!(h.len(), 0);

        h.insert("c", &[0.0, 0.0, 1.0]).unwrap();
        let matches = h.search(&[0.0, 0.0, 1.0], 1).unwrap();
        assert_eq!(matches[0].id, "c");
    }

    #[test]
    fn test_update_existing() {
        let h = new_test_hnsw(3);
        h.insert("a", &[1.0, 0.0, 0.0]).unwrap();
        h.insert("b", &[0.0, 1.0, 0.0]).unwrap();
        h.insert("a", &[0.0, 0.0, 1.0]).unwrap();

        assert_eq!(h.len(), 2);

        let matches = h.search(&[0.0, 0.0, 1.0], 1).unwrap();
        assert_eq!(matches[0].id, "a");
    }

    #[test]
    fn test_search_empty() {
        let h = new_test_hnsw(3);
        let matches = h.search(&[1.0, 0.0, 0.0], 5).unwrap();
        assert!(matches.is_empty());
    }

    #[test]
    fn test_search_top_k_zero() {
        let h = new_test_hnsw(3);
        h.insert("a", &[1.0, 0.0, 0.0]).unwrap();
        let matches = h.search(&[1.0, 0.0, 0.0], 0).unwrap();
        assert!(matches.is_empty());
    }

    #[test]
    fn test_single_node() {
        let h = new_test_hnsw(3);
        h.insert("only", &[0.5, 0.5, 0.5]).unwrap();
        let matches = h.search(&[1.0, 0.0, 0.0], 5).unwrap();
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].id, "only");
    }

    #[test]
    #[should_panic]
    fn test_panics_on_zero_dim() {
        HNSW::new(HNSWConfig {
            dim: 0,
            m: 16,
            ef_construction: 200,
            ef_search: 50,
        });
    }

    #[test]
    fn test_recall() {
        use rand::Rng;

        let dim = 32;
        let n = 2000;
        let queries = 50;
        let top_k = 10;

        let mut rng = rand::thread_rng();

        let h = HNSW::new(HNSWConfig {
            dim,
            m: 16,
            ef_construction: 128,
            ef_search: 64,
        });

        let mut ids = Vec::with_capacity(n);
        let mut vecs = Vec::with_capacity(n);
        for i in 0..n {
            let id = format!("v-{i}");
            let v = rand_unit_vec(&mut rng, dim);
            h.insert(&id, &v).unwrap();
            ids.push(id);
            vecs.push(v);
        }

        let mut total_recall = 0.0;
        for _ in 0..queries {
            let query = rand_unit_vec(&mut rng, dim);

            // Brute-force ground truth.
            let mut truth: Vec<(usize, f32)> = vecs
                .iter()
                .enumerate()
                .map(|(i, v)| (i, cosine_distance(&query, v)))
                .collect();
            truth.sort_by(|a, b| a.1.partial_cmp(&b.1).unwrap());
            let truth_set: HashSet<String> =
                truth.iter().take(top_k).map(|(i, _)| ids[*i].clone()).collect();

            let matches = h.search(&query, top_k).unwrap();
            let hits = matches.iter().filter(|m| truth_set.contains(&m.id)).count();
            total_recall += hits as f64 / top_k as f64;
        }

        let avg_recall = total_recall / queries as f64;
        assert!(
            avg_recall >= 0.80,
            "recall {avg_recall:.3} is below 0.80 threshold"
        );
    }

    fn rand_unit_vec(rng: &mut impl rand::Rng, dim: usize) -> Vec<f32> {
        let v: Vec<f32> = (0..dim).map(|_| rng.r#gen::<f32>() - 0.5).collect();
        let norm: f64 = v.iter().map(|&x| (x as f64) * (x as f64)).sum::<f64>().sqrt();
        if norm > 0.0 {
            v.into_iter().map(|x| x / norm as f32).collect()
        } else {
            v
        }
    }
}
