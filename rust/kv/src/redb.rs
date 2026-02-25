//! Redb-based persistent key-value store implementation.

use std::path::Path;

use redb::{Database, ReadableTable, TableDefinition};

use crate::{KVError, KVResult, KVStore};

const TABLE: TableDefinition<&str, &[u8]> = TableDefinition::new("kv");

/// A persistent key-value store backed by redb.
pub struct RedbStore {
    db: Database,
}

impl RedbStore {
    /// Open or create a redb store at the given path.
    pub fn open<P: AsRef<Path>>(path: P) -> KVResult<Self> {
        let db = Database::create(path).map_err(|e| KVError::Storage(e.to_string()))?;

        // Create the table if it doesn't exist
        let tx = db
            .begin_write()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        {
            let _ = tx
                .open_table(TABLE)
                .map_err(|e| KVError::Storage(e.to_string()))?;
        }
        tx.commit().map_err(|e| KVError::Storage(e.to_string()))?;

        Ok(Self { db })
    }
}

impl KVStore for RedbStore {
    fn get(&self, key: &str) -> KVResult<Option<Vec<u8>>> {
        let tx = self
            .db
            .begin_read()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        let table = tx
            .open_table(TABLE)
            .map_err(|e| KVError::Storage(e.to_string()))?;

        match table
            .get(key)
            .map_err(|e| KVError::Storage(e.to_string()))?
        {
            Some(value) => Ok(Some(value.value().to_vec())),
            None => Ok(None),
        }
    }

    fn set(&self, key: &str, value: &[u8]) -> KVResult<()> {
        let tx = self
            .db
            .begin_write()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        {
            let mut table = tx
                .open_table(TABLE)
                .map_err(|e| KVError::Storage(e.to_string()))?;
            table
                .insert(key, value)
                .map_err(|e| KVError::Storage(e.to_string()))?;
        }
        tx.commit().map_err(|e| KVError::Storage(e.to_string()))?;
        Ok(())
    }

    fn delete(&self, key: &str) -> KVResult<()> {
        let tx = self
            .db
            .begin_write()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        {
            let mut table = tx
                .open_table(TABLE)
                .map_err(|e| KVError::Storage(e.to_string()))?;
            table
                .remove(key)
                .map_err(|e| KVError::Storage(e.to_string()))?;
        }
        tx.commit().map_err(|e| KVError::Storage(e.to_string()))?;
        Ok(())
    }

    fn scan(&self, prefix: &str) -> KVResult<Vec<(String, Vec<u8>)>> {
        let tx = self
            .db
            .begin_read()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        let table = tx
            .open_table(TABLE)
            .map_err(|e| KVError::Storage(e.to_string()))?;

        let mut results = Vec::new();
        for item in table.iter().map_err(|e| KVError::Storage(e.to_string()))? {
            let (key, value) = item.map_err(|e| KVError::Storage(e.to_string()))?;
            let key_str = key.value();
            if key_str.starts_with(prefix) {
                results.push((key_str.to_string(), value.value().to_vec()));
            }
        }

        results.sort_by(|a, b| a.0.cmp(&b.0));
        Ok(results)
    }

    fn batch_set(&self, entries: &[(&str, &[u8])]) -> KVResult<()> {
        let tx = self
            .db
            .begin_write()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        {
            let mut table = tx
                .open_table(TABLE)
                .map_err(|e| KVError::Storage(e.to_string()))?;
            for (key, value) in entries {
                table
                    .insert(*key, *value)
                    .map_err(|e| KVError::Storage(e.to_string()))?;
            }
        }
        tx.commit().map_err(|e| KVError::Storage(e.to_string()))?;
        Ok(())
    }

    fn batch_delete(&self, keys: &[&str]) -> KVResult<()> {
        let tx = self
            .db
            .begin_write()
            .map_err(|e| KVError::Storage(e.to_string()))?;
        {
            let mut table = tx
                .open_table(TABLE)
                .map_err(|e| KVError::Storage(e.to_string()))?;
            for key in keys {
                table
                    .remove(*key)
                    .map_err(|e| KVError::Storage(e.to_string()))?;
            }
        }
        tx.commit().map_err(|e| KVError::Storage(e.to_string()))?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    #[test]
    fn test_redb_basic() {
        let dir = tempdir().unwrap();
        let store = RedbStore::open(dir.path().join("test.redb")).unwrap();

        store.set("key1", b"value1").unwrap();
        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));

        store.delete("key1").unwrap();
        assert_eq!(store.get("key1").unwrap(), None);
    }

    #[test]
    fn test_redb_scan() {
        let dir = tempdir().unwrap();
        let store = RedbStore::open(dir.path().join("test.redb")).unwrap();

        store.set("prefix:a", b"1").unwrap();
        store.set("prefix:b", b"2").unwrap();
        store.set("other:c", b"3").unwrap();

        let results = store.scan("prefix:").unwrap();
        assert_eq!(results.len(), 2);
    }
}
