use crate::{memory::HostPtr, state::Error as StateError};
use borsh::{from_slice, to_vec, BorshDeserialize, BorshSerialize};
use std::{
    cell::RefCell,
    collections::{hash_map::Entry, HashMap},
    hash::Hash,
};

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

#[derive(Debug, PartialEq)]
pub enum CachedData {
    Data(Vec<u8>),
    Deleted,
    ToDelete,
}

pub struct State<'a, K: Key> {
    cache: &'a RefCell<HashMap<K, CachedData>>,
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
    pub fn new(cache: &'a RefCell<HashMap<K, CachedData>>) -> Self {
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
        self.cache
            .borrow_mut()
            .insert(key, CachedData::Data(serialized));

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
    pub fn get<V>(self, key: K) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
        match self.cache.borrow_mut().entry(key) {
            Entry::Occupied(entry) => match entry.get() {
                CachedData::Data(data) => from_slice::<V>(data)
                    .map_err(|_| StateError::Deserialization)
                    .map(Some),
                CachedData::ToDelete | CachedData::Deleted => Ok(None),
            },
            Entry::Vacant(entry) => {
                if let Some(bytes) = Self::get_from_host(key)? {
                    let value = from_slice::<V>(&bytes)
                        .map_err(|_| StateError::Deserialization)
                        .map(Some);
                    entry.insert(CachedData::Data(bytes));
                    value
                } else {
                    entry.insert(CachedData::Deleted);
                    Ok(None)
                }
            }
        }
    }

    /// # Errors
    /// Returns an [Error] if the key cannot be serialized,
    /// if the host fails to handle the operation,
    /// or if the bytes cannot be borsh deserialized.
    pub fn delete<V>(self, key: K) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
        match self.cache.borrow_mut().entry(key) {
            Entry::Occupied(mut entry) => match entry.get_mut() {
                CachedData::Data(data) => {
                    let value = from_slice::<V>(data)
                        .map_err(|_| StateError::Deserialization)
                        .map(Some);
                    entry.insert(CachedData::Deleted);
                    value
                }
                CachedData::Deleted | CachedData::ToDelete => Ok(None),
            },
            Entry::Vacant(entry) => {
                if let Some(data) = Self::get_from_host(key)? {
                    entry.insert(CachedData::ToDelete);
                    from_slice::<V>(&data)
                        .map_err(|_| StateError::Deserialization)
                        .map(Some)
                } else {
                    entry.insert(CachedData::Deleted);
                    Ok(None)
                }
            }
        }
    }

    fn get_from_host(key: K) -> Result<Option<Vec<u8>>, Error> {
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

        Ok(Some(ptr.into()))
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

        let (mut puts, mut deletes) = (Vec::new(), Vec::new());
        self.cache
            .borrow_mut()
            .drain()
            .for_each(|(key, value)| match value {
                CachedData::Data(value) => puts.push(PutArgs { key, value }),
                CachedData::ToDelete => deletes.push(key),
                CachedData::Deleted => (),
            });

        if !puts.is_empty() {
            let serialized_args = borsh::to_vec(&puts).expect("failed to serialize");
            unsafe { put_many_bytes(serialized_args.as_ptr(), serialized_args.len()) };
        }

        if !deletes.is_empty() {
            let serialized_args = borsh::to_vec(&deletes.as_slice()).expect("failed to serialize");
            unsafe { delete_many_bytes(serialized_args.as_ptr(), serialized_args.len()) };
        }
    }
}
