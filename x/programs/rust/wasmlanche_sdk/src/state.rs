use serde::{de::DeserializeOwned, Serialize};
use serde_bare::{from_slice, to_vec};

use crate::{
    errors::StateError,
    host::{get_bytes, len_bytes, put_bytes},
    program::Program,
};

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
        V: Serialize,
        K: Into<Key>, 
    {
        let value_bytes = to_vec(value).map_err(|_| StateError::Serialization)?;
        match unsafe {
            put_bytes(
                &self.program,
                key.into(),
                value_bytes.as_ptr(),
                value_bytes.len(),
            )
        } {
            0 => Ok(()),
            _ => Err(StateError::Write),
        }
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
        T: DeserializeOwned,
    {
        let key : Key = key.into();
        let val_len = unsafe { len_bytes(&self.program, &key) };
        let val_ptr = unsafe { get_bytes(&self.program, key, val_len) };
        if val_ptr < 0 {
            return Err(StateError::Read);
        }

        // Rust takes ownership here so all of the above pointers will be freed on return (drop).
        let val = unsafe {
            Vec::from_raw_parts(
                val_ptr as *mut u8,
                val_len.try_into().expect("conversion from i32"),
                val_len.try_into().expect("conversion from i32"),
            )
        };
        from_slice(&val).map_err(|_| StateError::InvalidBytes)
    }
}




#[derive(Debug, Default, Clone)]
pub struct Key(pub Vec<u8>);

impl Key {
    #[must_use]
    pub fn new(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }

    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }

    #[must_use]
    pub fn len(&self) -> usize {
        self.0.len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

// implement from Vec<u8> for Key
impl From<Vec<u8>> for Key {
    fn from(value: Vec<u8>) -> Self {
        Self(value)
    }
}