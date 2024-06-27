use crate::{memory::HostPtr, state::Error as StateError};
use borsh::{from_slice, to_vec, BorshDeserialize, BorshSerialize};
use std::{cell::RefCell, collections::HashMap, hash::Hash};

#[derive(Clone, thiserror::Error, Debug)]
pub enum Error {
    #[error("invalid byte length: {0}")]
    InvalidByteLength(usize),

    #[error("failed to serialize bytes")]
    Serialization,

    #[error("failed to deserialize bytes")]
    Deserialization,
}

pub struct State<'a, K: Key + StateSchema> {
    cache: &'a RefCell<HashMap<K, Option<Vec<u8>>>>,
}

/// Key trait for program state keys
/// # Safety
/// This trait should only be implemented using the [`state_keys`](crate::state_keys) macro.
pub unsafe trait Key: Copy + PartialEq + Eq + Hash + BorshSerialize {}

pub trait StateSchema {
    type SchemaType: BorshSerialize + BorshDeserialize;
}

impl<'a, K: Key + StateSchema> Drop for State<'a, K> {
    fn drop(&mut self) {
        if !self.cache.borrow().is_empty() {
            // force flush
            self.flush();
        }
    }
}

impl<'a, K: Key + StateSchema> State<'a, K> {
    #[must_use]
    pub fn new(cache: &'a RefCell<HashMap<K, Option<Vec<u8>>>>) -> Self {
        Self { cache }
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [Error] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store(self, key: K, value: &K::SchemaType) -> Result<(), Error> {
        let serialized = to_vec(&value)
            .map_err(|_| StateError::Deserialization)
            .and_then(|bytes| {
                if bytes.is_empty() {
                    Err(StateError::InvalidByteLength(0))
                } else {
                    Ok(bytes)
                }
            })?;
        self.cache.borrow_mut().insert(key, Some(serialized));

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
    pub fn get(self, key: K) -> Result<Option<K::SchemaType>, Error> {
        let mut cache = self.cache.borrow_mut();
        let val_bytes = match cache.get(&key) {
            Some(Some(val)) => {
                if val.is_empty() {
                    return Ok(None);
                }

                val
            }
            Some(None) => return Ok(None),
            None => {
                if let Some(val) = Self::get_host(key)? {
                    cache.entry(key).or_insert(Some(val)).as_ref().unwrap()
                } else {
                    cache.insert(key, None);
                    return Ok(None);
                }
            }
        };

        from_slice::<K::SchemaType>(val_bytes)
            .map_err(|_| StateError::Deserialization)
            .map(Some)
    }

    fn get_host(key: K) -> Result<Option<Vec<u8>>, Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        let args_bytes = borsh::to_vec(&key).map_err(|_| StateError::Serialization)?;

        let ptr = unsafe { get_bytes(args_bytes.as_ptr(), args_bytes.len()) };

        if ptr.is_null() {
            Ok(None)
        } else {
            Ok(Some(ptr.into()))
        }
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the value is inexistent
    /// or if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<V: BorshDeserialize>(self, key: K) -> Result<Option<V>, Error> {
        let mut cache = self.cache.borrow_mut();
        let val_bytes = match cache.get(&key) {
            Some(Some(val)) => {
                if val.is_empty() {
                    cache.entry(key).or_insert(None);
                    return Ok(None);
                }

                val
            }
            Some(None) => return Ok(None),
            None => {
                &if let Some(val) = Self::get_host(key)? {
                    cache.entry(key).or_insert(Some(Vec::new()));
                    val
                } else {
                    return Ok(None);
                }
            }
        };

        from_slice::<V>(val_bytes)
            .map_err(|_| StateError::Deserialization)
            .map(Some)
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
