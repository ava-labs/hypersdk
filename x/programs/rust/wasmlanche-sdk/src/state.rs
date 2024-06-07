use crate::{memory::HostPtr, state::Error as StateError};
use borsh::{from_slice, to_vec, BorshDeserialize, BorshSerialize};
use std::cell::RefMut;
use std::collections::hash_map::Entry;
use std::collections::HashSet;
use std::{cell::RefCell, collections::HashMap, hash::Hash};

#[derive(Clone, thiserror::Error, Debug)]
pub enum Error {
    #[error("an unclassified error has occurred: {0}")]
    Other(String),

    #[error("invalid byte format")]
    InvalidBytes,

    #[error("invalid byte length: {0}")]
    InvalidByteLength(usize),

    #[error("invalid pointer offset")]
    InvalidPointer,

    #[error("invalid tag: {0}")]
    InvalidTag(u8),

    #[error("failed to write to host storage")]
    Write,

    #[error("failed to read from host storage")]
    Read,

    #[error("failed to serialize bytes")]
    Serialization,

    #[error("failed to deserialize bytes")]
    Deserialization,

    #[error("failed to convert integer")]
    IntegerConversion,

    #[error("failed to delete from host storage")]
    Delete,
}

#[derive(Debug, Default)]
pub struct Cache<K = ()> {
    puts: HashMap<K, Vec<u8>>,
    deletes: HashSet<K>,
}

impl<K> Cache<K> {
    pub fn new() -> Self {
        Cache {
            puts: HashMap::new(),
            deletes: HashSet::new(),
        }
    }
}

impl<K: Key> Cache<K> {
    fn is_empty(&self) -> bool {
        self.puts.is_empty() && self.deletes.is_empty()
    }

    fn get(&mut self, k: &K) -> Option<Option<&Vec<u8>>> {
        self.puts
            .get(k)
            .map(|data| Some(data))
            .or_else(|| self.deletes.get(k).map_or(None, |_| Some(None)))
    }

    fn put(&mut self, k: K, v: Vec<u8>) {
        self.deletes.remove(&k);
        self.puts.insert(k, v);
    }

    fn delete(&mut self, k: K) -> Option<Vec<u8>> {
        let v = self.puts.remove(&k);
        self.deletes.insert(k);
        v
    }

    fn entry(&mut self, k: K) -> Entry<K, Vec<u8>> {
        self.puts.entry(k)
    }
}

pub struct State<'a, K: Key> {
    cache: &'a RefCell<Cache<K>>,
}

/// # Safety
/// This trait should only be implemented using the [`state_keys`](crate::state_keys) macro.
pub unsafe trait Key: Copy + PartialEq + Eq + Hash + BorshSerialize {}

impl<'a, K: Key> Drop for State<'a, K> {
    fn drop(&mut self) {
        if !self.cache.borrow().is_empty() {
            // force flush
            self.flush();
        }
    }
}

impl<'a, K: Key> State<'a, K> {
    #[must_use]
    pub fn new(cache: &'a RefCell<Cache<K>>) -> Self {
        Self { cache }
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [Error] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<V>(self, key: K, value: &V) -> Result<(), Error>
    where
        V: BorshSerialize,
    {
        let serialized = to_vec(&value).map_err(|_| StateError::Deserialization)?;
        self.cache.borrow_mut().put(key, serialized);

        Ok(())
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<T: BorshDeserialize>(self, key: K) -> Result<Option<T>, Error> {
        let mut cache = self.cache.borrow_mut();

        if let Some(bytes) = if let Some(cached_bytes) = &cache.delete(key) {
            Some(cached_bytes)
        } else {
            Self::get_from_host::<K>(&mut cache, key)?
        } {
            Ok(Some(
                from_slice(bytes).map_err(|_| StateError::Deserialization)?,
            ))
        } else {
            Ok(None)
        }
    }

    /// Get a value from the host's storage.
    ///
    /// Note: The pointer passed to the host are only valid for the duration of this
    /// function call. This function will take ownership of the pointer and free it.
    ///
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized or if
    /// the host fails to read the key and value.
    /// # Panics
    /// Panics if the value cannot be converted from i32 to usize.
    pub fn get<V>(self, key: K) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
        let mut cache = self.cache.borrow_mut();
        let val_bytes = match cache.get(&key) {
            Some(Some(val)) => val,
            Some(None) => return Ok(None),
            None => match Self::get_from_host::<K>(&mut cache, key)? {
                Some(bytes) => bytes,
                None => return Ok(None),
            },
        };

        from_slice::<V>(val_bytes)
            .map_err(|_| StateError::Deserialization)
            .map(Some)
    }

    pub fn get_from_host<'b, V>(
        cache: &'b mut RefMut<Cache<K>>,
        key: K,
    ) -> Result<Option<&'b Vec<u8>>, Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        let args_bytes = borsh::to_vec(&key).map_err(|_| StateError::Serialization)?;

        let ptr = unsafe { get_bytes(args_bytes.as_ptr(), args_bytes.len()) };

        if ptr.is_null() {
            return Ok(None);
        }

        Ok(Some(cache.entry(key).or_insert(ptr.into())))
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    fn flush(&self) {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put_many"]
            fn put_many_bytes(ptr: *const u8, len: usize);

            #[link_name = "delete_many"]
            fn delete_many_bytes(ptr: *const u8, len: usize);
        }

        #[derive(BorshSerialize)]
        struct PutArgs<Key> {
            key: Key,
            value: Vec<u8>,
        }

        let mut cache = self.cache.borrow_mut();

        let args: Vec<_> = cache
            .puts
            .drain()
            .map(|(key, value)| PutArgs { key, value })
            .collect();
        let serialized_args = borsh::to_vec(&args).expect("failed to serialize");
        unsafe { put_many_bytes(serialized_args.as_ptr(), serialized_args.len()) };

        let args: Vec<_> = cache.deletes.drain().collect();
        let serialized_args = borsh::to_vec(&args).expect("failed to serialize");
        unsafe { delete_many_bytes(serialized_args.as_ptr(), serialized_args.len()) };
    }
}

#[cfg(test)]
mod tests {
    use super::{Cache, Key};

    impl Key for usize {}

    #[test]
    fn cache_operations() {
        let mut cache = Cache::new();
        assert!(cache.is_empty());
        assert!(cache.get(&0).is_none());
        assert!(cache.delete(0).is_none());
        assert_eq!(cache.get(&0), Some(None));
        cache.put(0, vec![1]);
        assert_eq!(cache.get(&0), Some(Some(&vec![1])));
    }
}
