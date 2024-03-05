use crate::{
    errors::StateError,
    host::{delete_bytes, get_bytes, put_bytes},
    memory::from_host_ptr,
    program::Program,
};
use borsh::{BorshDeserialize, BorshSerialize};
use std::ops::Deref;

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
    /// Returns an `StateError` if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<K, V>(&self, key: K, value: &V) -> Result<(), StateError>
    where
        V: BorshSerialize,
        K: Into<Key>,
    {
        unsafe { put_bytes(&self.program, &key.into(), value) }
    }

    /// Get a value from the host's storage.
    ///
    /// Note: The pointer passed to the host are only valid for the duration of this
    /// function call. This function will take ownership of the pointer and free it.
    ///
    /// # Errors
    /// Returns an `StateError` if the key cannot be serialized or if
    /// the host fails to read the key and value.
    /// # Panics
    /// Panics if the value cannot be converted from i32 to usize.
    pub fn get<T, K>(&self, key: K) -> Result<T, StateError>
    where
        K: Into<Key>,
        T: BorshDeserialize,
    {
        let val_ptr = unsafe { get_bytes(&self.program, &key.into())? };
        if val_ptr < 0 {
            return Err(StateError::Read);
        }

        // Wrap in OK for now, change from_raw_ptr to return Result
        unsafe { from_host_ptr(val_ptr) }
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an `StateError` if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<K>(&self, key: K) -> Result<(), StateError>
    where
        K: Into<Key>,
    {
        unsafe { delete_bytes(&self.program, &key.into()) }
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
