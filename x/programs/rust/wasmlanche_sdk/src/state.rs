use serde::{de::DeserializeOwned, Serialize};
use serde_bare::{to_vec};

use crate::{
    errors::StateError,
    host::{get_bytes, len_bytes, put_bytes},
    program::Program,
};

#[derive(Debug, Copy, Clone)]
pub struct Key([u8; 8]);

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
    pub fn store<K: Into<Key>, V, const N: usize>(&self, key: K, value: ) -> Result<(), StateError>
    where
        V: AsRef<[u8]>,
    {
        let mut value = [0u8; N];
        value.copy_from_slice(value.as_ref());

        let storable = Storable {
            key: key.into(),
            value,
            value_type_prefix: 0,
        };
        
        let value_bytes = to_vec(value.as_ref()).map_err(|_| StateError::Serialization)?;
        match unsafe {
            put_bytes(
                &self.program,
                &storable,
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
    pub fn get<K, const N: usize>(&self, key: K) -> Result<Storable<N>, StateError>
    where
        K: AsRef<[u8]>,
    {
        let key_ptr = key.as_ref().as_ptr();
        let key_len = key.as_ref().len();

        let val_len = unsafe { len_bytes(&self.program, key_ptr, key_len) };
        let val_ptr = unsafe { get_bytes(&self.program, key_ptr, key_len, val_len) };
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


fn from_slice<const N: usize>(slice: &[u8]) -> Result<Storable<N>, StateError> {
    // We need at least 1 byte for the type_prefix, plus the size of the key, plus N bytes for the value.
    if slice.len() < 1 + std::mem::size_of::<Key>() + N {
        return Err(StateError::InvalidBytes);
    }

    let value_type_prefix = slice[0];

    // For simplicity, let's assume Key is of fixed size, say 8 bytes.
    let key_bytes = &slice[1..1 + std::mem::size_of::<Key>()];
    let key = Key::from_bytes(key_bytes.try_into().expect("Incorrect key size"));

    let value_bytes = &slice[1 + std::mem::size_of::<Key>()..];
    let mut value = [0u8; N];
    value.copy_from_slice(value_bytes);

    Ok(Storable {
        key,
        value,
        value_type_prefix,
    })
}

struct Storable<const N: usize> {
    key: Key,
    value: [u8; N],
    value_type_prefix: u8, 
}

impl Key {
    fn from_bytes(bytes: [u8; 8]) -> Self {
        Key(bytes)
    }
}

