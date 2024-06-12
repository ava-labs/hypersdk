use crate::{memory::HostPtr, state::Error as StateError};
use borsh::{from_slice, to_vec, BorshDeserialize, BorshSerialize};
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

pub struct State<'a, K: Key> {
    cache: &'a RefCell<HashMap<K, Vec<u8>>>,
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
    pub fn new(cache: &'a RefCell<HashMap<K, Vec<u8>>>) -> Self {
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
        self.cache.borrow_mut().insert(key, serialized);

        Ok(())
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
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        let mut cache = self.cache.borrow_mut();

        let val_bytes = if let Some(val) = cache.get(&key) {
            val
        } else {
            let args_bytes = borsh::to_vec(&key).map_err(|_| StateError::Serialization)?;

            let ptr = unsafe { get_bytes(args_bytes.as_ptr(), args_bytes.len()) };

            if ptr.is_null() {
                return Ok(None);
            }

            cache.entry(key).or_insert(ptr.into())
        };

        from_slice::<V>(val_bytes)
            .map_err(|_| StateError::Deserialization)
            .map(Some)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<T: BorshDeserialize>(self, key: K) -> Result<Option<T>, Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "delete"]
            fn delete(ptr: *const u8, len: usize) -> HostPtr;
        }

        // TODO:
        // we should actually cache deletes as well
        // to avoid cache misses after delete
        self.cache.borrow_mut().remove(&key);

        let args_bytes = borsh::to_vec(&key).map_err(|_| StateError::Serialization)?;

        let bytes = unsafe { delete(args_bytes.as_ptr(), args_bytes.len()) };

        if bytes.is_null() {
            return Ok(None);
        }

        from_slice::<T>(&bytes)
            .map_err(|_| StateError::Deserialization)
            .map(Some)
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    fn flush(&self) {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put_many"]
            fn put_many_bytes(ptr: *const u8, len: usize);
        }

        #[derive(BorshSerialize)]
        struct PutArgs<Key> {
            key: Key,
            value: Vec<u8>,
        }

        let mut cache = self.cache.borrow_mut();

        let args: Vec<_> = cache
            .drain()
            .map(|(key, value)| PutArgs { key, value })
            .collect();
        let serialized_args = borsh::to_vec(&args).expect("failed to serialize");
        unsafe { put_many_bytes(serialized_args.as_ptr(), serialized_args.len()) };
    }
}
