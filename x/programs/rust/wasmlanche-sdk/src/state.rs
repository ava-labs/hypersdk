use crate::{memory::from_host_ptr, program::Program};
use borsh::{BorshDeserialize, BorshSerialize};
use std::ops::Deref;

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

pub struct State {
    program: Program,
}

impl State {
    #[must_use]
    pub fn new(program: Program) -> Self {
        Self { program }
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [Error] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<K, V>(&self, key: K, value: &V) -> Result<(), Error>
    where
        V: BorshSerialize,
        K: Into<Key>,
    {
        unsafe { host::put_bytes(&self.program, &key.into(), value) }
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
    pub fn get<T, K>(&self, key: K) -> Result<T, Error>
    where
        K: Into<Key>,
        T: BorshDeserialize,
    {
        let val_ptr = unsafe { host::get_bytes(&self.program, &key.into())? };
        // TODO shouldn't we check if its null here instead ?
        if val_ptr < std::ptr::null() {
            return Err(Error::Read);
        }

        // Wrap in OK for now, change from_raw_ptr to return Result
        from_host_ptr(val_ptr)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<K>(&self, key: K) -> Result<(), Error>
    where
        K: Into<Key>,
    {
        unsafe { host::delete_bytes(&self.program, &key.into()) }
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

mod host {
    use super::{BorshSerialize, Key, Program};
    use crate::{memory::to_ffi_ptr, program::CPointer, state::Error};

    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "put"]
        fn _put(caller: CPointer, key: CPointer, value: CPointer) -> *const u8;

        #[link_name = "get"]
        fn _get(caller: CPointer, key: CPointer) -> *const u8;

        #[link_name = "delete"]
        fn _delete(caller: CPointer, key: CPointer) -> *const u8;
    }

    /// Persists the bytes at `value` at key on the host storage.
    pub(super) unsafe fn put_bytes<V>(caller: &Program, key: &Key, value: &V) -> Result<(), Error>
    where
        V: BorshSerialize,
    {
        let value_bytes = borsh::to_vec(value).map_err(|_| Error::Serialization)?;
        // prepend length to both key & value
        let caller = to_ffi_ptr(caller.id())?;
        let value = to_ffi_ptr(&value_bytes)?;
        let key = to_ffi_ptr(key)?;

        if unsafe { _put(caller, key, value) }.is_null() {
            Ok(())
        } else {
            Err(Error::Write)
        }
    }

    /// Gets the bytes associated with the key from the host.
    pub(super) unsafe fn get_bytes(caller: &Program, key: &Key) -> Result<*const u8, Error> {
        // prepend length to key
        let caller = to_ffi_ptr(caller.id())?;
        let key = to_ffi_ptr(key)?;
        Ok(unsafe { _get(caller, key) })
    }

    /// Deletes the bytes at key ptr from the host storage
    pub(super) unsafe fn delete_bytes(caller: &Program, key: &Key) -> Result<(), Error> {
        let caller = to_ffi_ptr(caller.id())?;
        let key = to_ffi_ptr(key)?;
        if unsafe { _delete(caller, key) }.is_null() {
            Ok(())
        } else {
            Err(Error::Delete)
        }
    }
}
