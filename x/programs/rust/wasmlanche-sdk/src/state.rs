use crate::{memory::into_bytes, state::Error as StateError};
use borsh::{from_slice, to_vec, BorshDeserialize, BorshSerialize};
use std::{collections::HashMap, hash::Hash, ops::Deref};

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

pub struct State<K>
where
    K: Into<Key> + Hash + PartialEq + Eq + Clone,
{
    cache: HashMap<K, Vec<u8>>,
}

impl<K> Drop for State<K>
where
    K: Into<Key> + Hash + PartialEq + Eq + Clone,
{
    fn drop(&mut self) {
        if !self.cache.is_empty() {
            // force flush
            self.flush().unwrap();
        }
    }
}

impl<K> Default for State<K>
where
    K: Into<Key> + Hash + PartialEq + Eq + Clone,
{
    fn default() -> Self {
        Self::new()
    }
}

impl<K> State<K>
where
    K: Into<Key> + Hash + PartialEq + Eq + Clone,
{
    #[must_use]
    pub fn new() -> Self {
        Self {
            cache: HashMap::new(),
        }
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [Error] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<V>(&mut self, key: K, value: &V) -> Result<(), Error>
    where
        V: BorshSerialize,
    {
        let serialized = to_vec(&value).map_err(|_| StateError::Deserialization)?;
        self.cache.insert(key, serialized);

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
    pub fn get<V>(&mut self, key: K) -> Result<V, Error>
    where
        V: BorshDeserialize,
    {
        let val_bytes = if let Some(val) = self.cache.get(&key) {
            val
        } else {
            let args = GetAndDeleteArgs {
                // TODO: shouldn't have to clone here
                key: key.clone().into().0,
            };

            let args_bytes = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

            let ptr = host::get_bytes(&args_bytes)?;

            let bytes = into_bytes(ptr).ok_or(Error::InvalidPointer)?;

            // TODO:
            // should be able to do something like the following
            // `let key = Key(args.key);`
            // to avoid cloning. The problem is we convert into a Key without knowing
            // that we can convert back into a K.
            // Either we need the key to actually be `Key` instead of `Into<Key>`
            // or we put the bound `K: From<Key>` as well.
            self.cache.entry(key).or_insert(bytes)
        };

        from_slice::<V>(val_bytes).map_err(|_| StateError::Deserialization)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<T: BorshDeserialize>(&mut self, key: K) -> Result<Option<T>, Error> {
        self.cache.remove(&key);

        let args = GetAndDeleteArgs { key: key.into().0 };

        let args_bytes = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

        host::delete_bytes(&args_bytes)
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    fn flush(&mut self) -> Result<(), Error> {
        let args_iter = self
            .cache
            .drain()
            .map(|(key, val)| (key.into(), val))
            .map(|(key, val)| PutArgs {
                key: key.0,
                bytes: val,
            })
            .map(|args| borsh::to_vec(&args).map_err(|_| StateError::Serialization));

        for args in args_iter {
            host::put_bytes(&args?)?;
        }

        Ok(())
    }
}

/// Key is a wrapper around a `Vec<u8>` that represents a key in the host storage.
#[derive(Debug, Default, Clone)]
pub struct Key(Vec<u8>);

impl Deref for Key {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl Key {
    /// Returns a new Key from the bytes.
    #[must_use]
    pub fn new(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }
}

#[derive(BorshSerialize)]
struct PutArgs {
    key: Vec<u8>,
    bytes: Vec<u8>,
}

#[derive(BorshSerialize)]
struct GetAndDeleteArgs {
    key: Vec<u8>,
}

mod host {
    use super::Error;
    use crate::memory::from_host_ptr;
    use borsh::BorshDeserialize;
    use std::ptr::NonNull;

    /// Persists the bytes at key on the host storage.
    pub(super) fn put_bytes(bytes: &[u8]) -> Result<(), Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put"]
            fn ffi(ptr: *const u8, len: usize) -> usize;
        }

        let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };

        match result {
            0 => Ok(()),
            _ => Err(Error::Write),
        }
    }

    /// Gets the bytes associated with the key from the host.
    pub(super) fn get_bytes(bytes: &[u8]) -> Result<*const u8, Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn ffi(ptr: *const u8, len: usize) -> *const u8;
        }

        let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };

        if result.is_null() {
            Err(Error::Read)
        } else {
            Ok(result)
        }
    }

    /// Deletes the bytes at key ptr from the host storage
    pub(super) fn delete_bytes<T: BorshDeserialize>(bytes: &[u8]) -> Result<Option<T>, Error> {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "delete"]
            fn ffi(ptr: *const u8, len: usize) -> NonNull<u8>;
        }

        let ptr = unsafe { ffi(bytes.as_ptr(), bytes.len()) };

        from_host_ptr(ptr.as_ptr())
    }
}
