use std::collections::HashMap;
use std::io::{BufReader, BufWriter, Read, Write};

use crate::error::VecError;
use crate::hnsw::{HNSWConfig, HNSW};

/// Binary format magic and version, matching Go's implementation.
const HNSW_MAGIC: [u8; 4] = [b'H', b'N', b'S', b'W'];
const HNSW_VERSION: u32 = 1;

/// Save serializes the HNSW index to a writer in a compact binary format.
///
/// The format is byte-level compatible with Go's `HNSW.Save`:
///
/// ```text
/// [4B magic "HNSW"] [4B version=1]
/// [4B dim] [4B M] [4B efConstruction] [4B efSearch]
/// [4B numSlots] [4B activeCount] [4B maxLevel] [4B entryID(int32)]
/// [4B freeCount] [freeCount x 4B free IDs]
/// For each slot:
///   [1B active flag]
///   If active:
///     [4B idLen] [idLen bytes ID string]
///     [4B level]
///     [dim x 4B float32 vector]
///     For each layer 0..level:
///       [4B numFriends] [numFriends x 4B friend IDs(uint32)]
/// ```
///
/// All multi-byte values are little-endian.
pub fn save(h: &HNSW, w: &mut dyn Write) -> Result<(), VecError> {
    let inner = h.read_inner();
    let mut bw = BufWriter::new(w);

    let write_err = |e: std::io::Error| VecError::Io(e.to_string());

    // Header.
    bw.write_all(&HNSW_MAGIC).map_err(write_err)?;
    bw.write_all(&HNSW_VERSION.to_le_bytes()).map_err(write_err)?;

    // Config.
    bw.write_all(&(inner.cfg.dim as u32).to_le_bytes()).map_err(write_err)?;
    bw.write_all(&(inner.cfg.m as u32).to_le_bytes()).map_err(write_err)?;
    bw.write_all(&(inner.cfg.ef_construction as u32).to_le_bytes()).map_err(write_err)?;
    bw.write_all(&(inner.cfg.ef_search as u32).to_le_bytes()).map_err(write_err)?;

    // Index metadata.
    bw.write_all(&(inner.nodes.len() as u32).to_le_bytes()).map_err(write_err)?;
    bw.write_all(&(inner.count as u32).to_le_bytes()).map_err(write_err)?;
    bw.write_all(&(inner.max_level as u32).to_le_bytes()).map_err(write_err)?;
    bw.write_all(&inner.entry_id.to_le_bytes()).map_err(write_err)?;

    // Free list.
    bw.write_all(&(inner.free.len() as u32).to_le_bytes()).map_err(write_err)?;
    for &f in &inner.free {
        bw.write_all(&f.to_le_bytes()).map_err(write_err)?;
    }

    // Nodes.
    for nd_opt in &inner.nodes {
        match nd_opt {
            None => {
                bw.write_all(&[0u8]).map_err(write_err)?;
            }
            Some(nd) => {
                bw.write_all(&[1u8]).map_err(write_err)?;

                // External ID.
                let id_bytes = nd.id.as_bytes();
                bw.write_all(&(id_bytes.len() as u32).to_le_bytes()).map_err(write_err)?;
                bw.write_all(id_bytes).map_err(write_err)?;

                // Level.
                bw.write_all(&(nd.level as u32).to_le_bytes()).map_err(write_err)?;

                // Vector.
                for &v in &nd.vector {
                    bw.write_all(&v.to_le_bytes()).map_err(write_err)?;
                }

                // Friend lists per layer.
                for lev in 0..=nd.level {
                    let friends = if lev < nd.friends.len() {
                        &nd.friends[lev]
                    } else {
                        &Vec::new() as &Vec<u32>
                    };
                    bw.write_all(&(friends.len() as u32).to_le_bytes()).map_err(write_err)?;
                    for &f in friends {
                        bw.write_all(&f.to_le_bytes()).map_err(write_err)?;
                    }
                }
            }
        }
    }

    bw.flush().map_err(write_err)?;
    Ok(())
}

/// Load deserializes an HNSW index from a reader.
///
/// The binary format must match what `save` produces (and Go's `HNSW.Save`).
/// Derived state (entryID, maxLevel, free list) is recomputed from actual
/// data — file metadata is read but not trusted.
pub fn load(r: &mut dyn Read) -> Result<HNSW, VecError> {
    let mut br = BufReader::new(r);
    let read_err = |e: std::io::Error| VecError::Io(e.to_string());

    let mut buf4 = [0u8; 4];

    // Magic.
    br.read_exact(&mut buf4).map_err(read_err)?;
    if buf4 != HNSW_MAGIC {
        return Err(VecError::InvalidFormat(format!(
            "invalid magic {:?}",
            buf4
        )));
    }

    // Version.
    br.read_exact(&mut buf4).map_err(read_err)?;
    let version = u32::from_le_bytes(buf4);
    if version != HNSW_VERSION {
        return Err(VecError::InvalidFormat(format!(
            "unsupported version {version} (want {HNSW_VERSION})"
        )));
    }

    // Config.
    let read_u32 = |br: &mut BufReader<&mut dyn Read>| -> Result<u32, VecError> {
        let mut buf = [0u8; 4];
        br.read_exact(&mut buf).map_err(|e| VecError::Io(e.to_string()))?;
        Ok(u32::from_le_bytes(buf))
    };

    let dim = read_u32(&mut br)? as usize;
    if dim == 0 {
        return Err(VecError::InvalidFormat(
            "invalid dimension 0".into(),
        ));
    }
    let m = read_u32(&mut br)? as usize;
    let ef_c = read_u32(&mut br)? as usize;
    let ef_s = read_u32(&mut br)? as usize;

    // Metadata — read but don't trust.
    let num_slots = read_u32(&mut br)? as usize;
    let _file_active_count = read_u32(&mut br)?;
    let _file_max_level = read_u32(&mut br)?;

    let mut buf_i32 = [0u8; 4];
    br.read_exact(&mut buf_i32).map_err(read_err)?;
    let _file_entry_id = i32::from_le_bytes(buf_i32);

    // Free list — read to advance stream, but discard; we rebuild.
    let free_count = read_u32(&mut br)? as usize;
    for _ in 0..free_count {
        let _ = read_u32(&mut br)?;
    }

    // Nodes.
    let mut nodes: Vec<Option<super::hnsw::HnswNode>> = Vec::with_capacity(num_slots);
    let mut id_map = HashMap::new();

    for i in 0..num_slots {
        let mut active_buf = [0u8; 1];
        br.read_exact(&mut active_buf).map_err(read_err)?;
        if active_buf[0] == 0 {
            nodes.push(None);
            continue;
        }

        // External ID.
        let id_len = read_u32(&mut br)? as usize;
        let mut id_bytes = vec![0u8; id_len];
        br.read_exact(&mut id_bytes).map_err(read_err)?;
        let id = String::from_utf8(id_bytes)
            .map_err(|e| VecError::InvalidFormat(e.to_string()))?;

        // Level.
        let level = read_u32(&mut br)? as usize;
        if level > 31 {
            return Err(VecError::InvalidFormat(format!(
                "node level {level} exceeds maximum 31"
            )));
        }

        // Vector.
        let mut vector = vec![0.0f32; dim];
        for v in &mut vector {
            let mut fb = [0u8; 4];
            br.read_exact(&mut fb).map_err(read_err)?;
            *v = f32::from_le_bytes(fb);
        }

        // Friends.
        let mut friends = Vec::with_capacity(level + 1);
        for _ in 0..=level {
            let nf = read_u32(&mut br)? as usize;
            let mut layer_friends = Vec::with_capacity(nf);
            for _ in 0..nf {
                let f_id = read_u32(&mut br)?;
                if f_id as usize >= num_slots {
                    return Err(VecError::InvalidFormat(format!(
                        "friend ID {f_id} out of bounds (numSlots={num_slots})"
                    )));
                }
                layer_friends.push(f_id);
            }
            friends.push(layer_friends);
        }

        id_map.insert(id.clone(), i as u32);
        nodes.push(Some(super::hnsw::HnswNode {
            id,
            vector,
            level,
            friends,
        }));
    }

    // Recompute derived state from actual data.
    let mut count = 0;
    let mut best_entry: i32 = -1;
    let mut best_level: i32 = -1;
    let mut free = Vec::new();
    for (i, nd) in nodes.iter().enumerate() {
        match nd {
            None => free.push(i as u32),
            Some(nd) => {
                count += 1;
                if nd.level as i32 > best_level {
                    best_entry = i as i32;
                    best_level = nd.level as i32;
                }
            }
        }
    }
    let max_level = if best_level >= 0 { best_level as usize } else { 0 };

    let mut cfg = HNSWConfig {
        dim,
        m,
        ef_construction: ef_c,
        ef_search: ef_s,
    };
    cfg.set_defaults();

    let level_mul = 1.0 / (cfg.m as f64).ln();

    Ok(HNSW::from_inner(super::hnsw::HnswInner {
        cfg,
        nodes,
        id_map,
        entry_id: best_entry,
        max_level,
        count,
        free,
        level_mul,
    }))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::vecstore::VecIndex;

    fn new_test_hnsw(dim: usize) -> HNSW {
        HNSW::new(HNSWConfig {
            dim,
            m: 8,
            ef_construction: 64,
            ef_search: 32,
        })
    }

    #[test]
    fn test_save_load() {
        let h = new_test_hnsw(4);
        h.insert("a", &[1.0, 0.0, 0.0, 0.0]).unwrap();
        h.insert("b", &[0.0, 1.0, 0.0, 0.0]).unwrap();
        h.insert("c", &[0.0, 0.0, 1.0, 0.0]).unwrap();
        h.delete("b").unwrap();

        let mut buf = Vec::new();
        save(&h, &mut buf).unwrap();

        let h2 = load(&mut buf.as_slice()).unwrap();
        assert_eq!(h2.len(), h.len());

        let query = [1.0f32, 0.0, 0.0, 0.0];
        let m1 = h.search(&query, 2).unwrap();
        let m2 = h2.search(&query, 2).unwrap();
        assert_eq!(m1.len(), m2.len());
        for (a, b) in m1.iter().zip(m2.iter()) {
            assert_eq!(a.id, b.id);
        }

        // Can insert into loaded index.
        h2.insert("d", &[0.0, 0.0, 0.0, 1.0]).unwrap();
        assert_eq!(h2.len(), 3);
    }

    #[test]
    fn test_save_load_empty() {
        let h = new_test_hnsw(4);

        let mut buf = Vec::new();
        save(&h, &mut buf).unwrap();

        let h2 = load(&mut buf.as_slice()).unwrap();
        assert_eq!(h2.len(), 0);

        h2.insert("a", &[1.0, 0.0, 0.0, 0.0]).unwrap();
        assert_eq!(h2.len(), 1);
    }

    #[test]
    fn test_load_invalid_magic() {
        let bad = b"NOPE";
        assert!(load(&mut bad.as_slice()).is_err());
    }
}
