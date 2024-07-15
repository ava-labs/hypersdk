use crate::{memory::HostPtr, types::Address};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};
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

pub struct State<'a, K: Key> {
    cache: &'a RefCell<HashMap<K, Option<Vec<u8>>>>,
}

/// Key trait for program state keys
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
    pub fn new(cache: &'a RefCell<HashMap<K, Option<Vec<u8>>>>) -> Self {
        Self { cache }
    }

    /// Store a list of tuple of key and value to the host storage.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<'b, V: BorshSerialize + 'b, Pairs: IntoIterator<Item = (K, &'b V)>>(
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
    pub fn store_by_key<V: BorshSerialize>(self, key: K, value: &V) -> Result<(), Error> {
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
    pub fn get<V>(self, key: K) -> Result<Option<V>, Error>
    where
        V: BorshDeserialize,
    {
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

        from_slice::<V>(val_bytes)
            .map_err(|_| Error::Deserialization)
            .map(Some)
    }

    fn get_host(key: K) -> Result<Option<Vec<u8>>, Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        let args_bytes = borsh::to_vec(&key).map_err(|_| Error::Serialization)?;

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
            .map_err(|_| Error::Deserialization)
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
