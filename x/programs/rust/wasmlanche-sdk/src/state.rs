use crate::{memory::HostPtr, types::Address};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};
use bytemuck::{NoUninit, Pod};
use std::{
    cell::RefCell,
    collections::{hash_map::Entry, HashMap},
    hash::Hash,
};

#[derive(Clone, thiserror::Error, Debug)]
pub enum Error {
    #[error("invalid byte length: {0}")]
    InvalidByteLength(usize),

    #[error("failed to serialize bytes")]
    Serialization,

    #[error("failed to deserialize bytes")]
    Deserialization,
}

/// Gets the balance for the specified address
/// # Panics
/// Panics if there was an issue deserializing the balance
#[must_use]
pub fn get_balance(account: Address) -> u64 {
    #[link(wasm_import_module = "balance")]
    extern "C" {
        #[link_name = "get"]
        fn get(ptr: *const u8, len: usize) -> HostPtr;
    }
    let ptr = borsh::to_vec(&account).expect("failed to serialize args");
    let bytes = unsafe { get(ptr.as_ptr(), ptr.len()) };

    borsh::from_slice(&bytes).expect("failed to deserialize the balance")
}

pub struct State<'a> {
    cache: &'a RefCell<HashMap<Vec<u8>, Option<Vec<u8>>>>,
}

/// Key trait for program state keys
/// # Safety
/// This trait should only be implemented using the [`state_keys`](crate::state_keys) macro.
pub unsafe trait Key: Copy + PartialEq + Eq + Hash + BorshSerialize {}

pub unsafe trait Schema {
    type Key: Pod;
    type Value: BorshSerialize + BorshDeserialize;

    fn get() -> Self::Value;
}

impl<'a> Drop for State<'a> {
    fn drop(&mut self) {
        if !self.cache.borrow().is_empty() {
            // force flush
            self.flush();
        }
    }
}

impl<'a> State<'a> {
    #[must_use]
    pub fn new(cache: &'a RefCell<HashMap<Vec<u8>, Option<Vec<u8>>>>) -> Self {
        Self { cache }
    }

    /// Store a list of tuple of key and value to the host storage.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<'b, V: BorshSerialize + 'b, Pairs: IntoIterator<Item = (Vec<u8>, &'b V)>>(
        self,
        pairs: Pairs,
    ) -> Result<(), Error> {
        let cache = &mut self.cache.borrow_mut();

        pairs
            .into_iter()
            .map(|(k, v)| borsh::to_vec(&v).map(|bytes| (k, Some(bytes))))
            .try_for_each(|result| {
                result.map(|(k, v)| {
                    cache.insert(k, v);
                })
            })
            .map_err(|_| Error::Serialization)?;

        Ok(())
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store_by_key<K: NoUninit, V: BorshSerialize>(
        self,
        key: &K,
        value: &V,
    ) -> Result<(), Error> {
        let key = bytemuck::bytes_of(key).to_vec();
        self.store([(key, value)])
    }

    /// Get a value from the host's storage.
    ///
    /// Note: The pointer passed to the host are only valid for the duration of this
    /// function call. This function will take ownership of the pointer and free it.
    ///
    /// # Errors
    /// Returns an [`Error`] if the key cannot be serialized or if
    /// the host fails to read the key and value.
    /// # Panics
    /// Panics if the value cannot be converted from i32 to usize.
    pub fn get<K: NoUninit, V>(self, key: &K) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
        self.get_mut_with(key, |x| from_slice(x))
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the value is inexistent
    /// or if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<K: NoUninit, V: BorshDeserialize>(self, key: &K) -> Result<Option<V>, Error> {
        self.get_mut_with(key, |val| from_slice(&std::mem::take(val)))
    }

    /// Only used internally
    /// The closure is only called if the key already exists in the cache
    /// The closure may mutate the value and return anything it likes for deserializaiton
    fn get_mut_with<K: NoUninit, V, F>(self, key: &K, f: F) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
        F: FnOnce(&mut Vec<u8>) -> borsh::io::Result<V>,
    {
        let mut cache = self.cache.borrow_mut();
        // TODO: should only need to allocate on cache-miss
        let key = bytemuck::bytes_of(key).to_vec();

        let cache_entry = match cache.entry(key) {
            Entry::Occupied(entry) => entry.into_mut(),
            Entry::Vacant(entry) => {
                let bytes = {
                    let key = entry.key();
                    get_bytes(&key)
                };
                entry.insert(bytes)
            }
        };

        match cache_entry {
            None => Ok(None),
            Some(val) if val.is_empty() => Ok(None),
            Some(val) => f(val).map_err(|_| Error::Deserialization).map(Some),
        }
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    fn flush(&self) {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put"]
            fn put(ptr: *const u8, len: usize);
        }

        #[derive(BorshSerialize)]
        struct PutArgs<Key> {
            key: Key,
            value: Vec<u8>,
        }

        let mut cache = self.cache.borrow_mut();

        let args: Vec<_> = cache
            .drain()
            .filter_map(|(key, val)| val.map(|value| PutArgs { key, value }))
            .collect();

        if !args.is_empty() {
            let serialized_args = borsh::to_vec(&args).expect("failed to serialize");
            unsafe { put(serialized_args.as_ptr(), serialized_args.len()) };
        }
    }
}

fn get_bytes(key: &[u8]) -> Option<Vec<u8>> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "get"]
        fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
    }

    let ptr = unsafe { get_bytes(key.as_ptr(), key.len()) };

    if ptr.is_null() {
        None
    } else {
        Some(ptr.into())
    }
}
