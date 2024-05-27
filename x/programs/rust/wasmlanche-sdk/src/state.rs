use crate::{memory::into_bytes, state::Error as StateError};
use borsh::{from_slice, to_vec, BorshDeserialize, BorshSerialize};
use std::{cell::RefCell, collections::HashMap, hash::Hash, io::ErrorKind};

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

pub trait Key: Copy + PartialEq + Eq + Hash {
    fn as_prefixed(&self) -> PrefixedBytes<'_>;
}

impl<'a, K: Key> Drop for State<'a, K> {
    fn drop(&mut self) {
        if !self.cache.borrow().is_empty() {
            // force flush
            self.flush().unwrap();
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
    pub fn get<V>(self, key: K) -> Result<V, Error>
    where
        V: BorshDeserialize,
    {
        let mut cache = self.cache.borrow_mut();

        let val_bytes = if let Some(val) = cache.get(&key) {
            val
        } else {
            let args = GetAndDeleteArgs {
                key: key.as_prefixed(),
            };

            let args_bytes = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

            let ptr = host::get_bytes(&args_bytes)?;

            let bytes = into_bytes(ptr).ok_or(Error::InvalidPointer)?;

            cache.entry(key).or_insert(bytes)
        };

        from_slice::<V>(val_bytes).map_err(|_| StateError::Deserialization)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<T: BorshDeserialize>(self, key: K) -> Result<Option<T>, Error> {
        // TODO:
        // we should actually cache deletes as well
        // to avoid cache misses after delete
        self.cache.borrow_mut().remove(&key);

        let args = GetAndDeleteArgs {
            key: key.as_prefixed(),
        };

        let args_bytes = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

        host::delete_bytes(&args_bytes)
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    fn flush(&self) -> Result<(), Error> {
        let mut cache = self.cache.borrow_mut();

        let args_iter = cache.drain().map(|(key, value)| {
            borsh::to_vec(&PutArgs {
                key: key.as_prefixed(),
                value,
            })
            .map_err(|_| StateError::Serialization)
        });

        for args in args_iter {
            host::put_bytes(&args?)?;
        }

        Ok(())
    }
}

pub struct PrefixedBytes<'a>(u8, &'a [u8]);

impl<'a> PrefixedBytes<'a> {
    #[must_use]
    pub fn new(prefix: u8, bytes: &'a [u8]) -> Self {
        Self(prefix, bytes)
    }
}

impl BorshSerialize for PrefixedBytes<'_> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self(prefix, bytes) = self;
        let len = 1 + u32::try_from(bytes.len()).map_err(|_| ErrorKind::InvalidData)?;

        // TODO: just use bytemuck with the enum
        writer.write_all(&len.to_le_bytes())?;
        writer.write_all(&[*prefix])?;
        writer.write_all(bytes)?;

        Ok(())
    }
}

pub struct PrefixedMultiBytes<'a>(u8, &'a [&'a [u8]]);

impl<'a> PrefixedMultiBytes<'a> {
    #[must_use]
    pub fn new(prefix: u8, bytes: &'a [&'a [u8]]) -> Self {
        Self(prefix, bytes)
    }
}

impl BorshSerialize for PrefixedMultiBytes<'_> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self(prefix, bytes) = self;

        let len = 1 + u32::try_from(bytes.iter().map(|byte| (byte.len())).sum::<usize>())
            .map_err(|_| ErrorKind::InvalidData)?;

        // TODO: just use bytemuck with the enum
        writer.write_all(&len.to_le_bytes())?;
        writer.write_all(&[*prefix])?;
        bytes.iter().try_for_each(|buf| writer.write_all(buf))
    }
}

#[derive(BorshSerialize)]
struct GetAndDeleteArgs<'a> {
    key: PrefixedBytes<'a>,
}

#[derive(BorshSerialize)]
struct PutArgs<'a> {
    key: PrefixedBytes<'a>,
    value: Vec<u8>,
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
