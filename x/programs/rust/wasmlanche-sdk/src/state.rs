use crate::{from_host_ptr, program::Program, state::Error as StateError};
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
    program: Program,
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

impl<K> State<K>
where
    K: Into<Key> + Hash + PartialEq + Eq + Clone,
{
    #[must_use]
    pub fn new(program: Program) -> Self {
        Self {
            program,
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
            let val_ptr = unsafe { host::get_bytes(&self.program, &key.clone().into())? };
            if val_ptr < 0 {
                return Err(Error::Read);
            }

            // TODO Wrap in OK for now, change from_raw_ptr to return Result
            let bytes = from_host_ptr(val_ptr)?;
            self.cache.entry(key).or_insert(bytes)
        };

        from_slice::<V>(val_bytes).map_err(|_| StateError::Deserialization)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete(&mut self, key: K) -> Result<(), Error> {
        self.cache.remove(&key);

        unsafe { host::delete_bytes(&self.program, &key.into()) }
    }

    /// Apply all pending operations to storage and mark the cache as flushed
    fn flush(&mut self) -> Result<(), Error> {
        for (key, value) in self.cache.drain() {
            unsafe {
                host::put_bytes(&self.program, &key.into(), &value)?;
            }
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

macro_rules! ffi_linker {
    ($mod:literal, $link:literal, $caller:ident, $key:ident) => {
        #[link(wasm_import_module = $mod)]
        extern "C" {
            #[link_name = $link]
            fn ffi(caller: i64, key: i64) -> i64;
        }

        let $caller = to_host_ptr($caller.id())?;
        let $key = to_host_ptr($key)?;
    };
    ($mod:literal, $link:literal, $caller:ident, $key:ident, $value:ident) => {
        #[link(wasm_import_module = $mod)]
        extern "C" {
            #[link_name = $link]
            fn ffi(caller: i64, key: i64, value: i64) -> i64;
        }

        let $caller = to_host_ptr($caller.id())?;
        let $key = to_host_ptr($key)?;
        let value_bytes = borsh::to_vec($value).map_err(|_| Error::Serialization)?;
        let $value = to_host_ptr(&value_bytes)?;
    };
}

macro_rules! call_host_fn {
    (
        wasm_import_module = $mod:literal
        link_name = $link:literal
        args = ($caller:ident, $key:ident)
    ) => {{
        ffi_linker!($mod, $link, $caller, $key);

        unsafe { ffi($caller, $key) }
    }};

    (
        wasm_import_module = $mod:literal
        link_name = $link:literal
        args = ($caller:ident, $key:ident, $value:ident)
    ) => {{
        ffi_linker!($mod, $link, $caller, $key, $value);

        unsafe { ffi($caller, $key, $value) }
    }};
}

mod host {
    use super::{BorshSerialize, Key, Program};
    use crate::{memory::to_host_ptr, state::Error};

    /// Persists the bytes at `value` at key on the host storage.
    pub(super) unsafe fn put_bytes<V>(caller: &Program, key: &Key, value: &V) -> Result<(), Error>
    where
        V: BorshSerialize,
    {
        match call_host_fn! {
            wasm_import_module = "state"
            link_name = "put"
            args = (caller, key, value)
        } {
            0 => Ok(()),
            _ => Err(Error::Write),
        }
    }

    /// Gets the bytes associated with the key from the host.
    pub(super) unsafe fn get_bytes(caller: &Program, key: &Key) -> Result<i64, Error> {
        Ok(call_host_fn! {
            wasm_import_module = "state"
                link_name = "get"
                args = (caller, key)
        })
    }

    /// Deletes the bytes at key ptr from the host storage
    pub(super) unsafe fn delete_bytes(caller: &Program, key: &Key) -> Result<(), Error> {
        match call_host_fn! {
            wasm_import_module = "state"
            link_name = "delete"
            args = (caller, key)
        } {
            0 => Ok(()),
            _ => Err(Error::Delete),
        }
    }
}
