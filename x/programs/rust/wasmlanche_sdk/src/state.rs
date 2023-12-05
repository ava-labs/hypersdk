use crate::{
    errors::StateError,
    host::{get_bytes, put_bytes},
    program::Program,
};
use borsh::{from_slice, BorshDeserialize, BorshSerialize};

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
        let val_ptr = unsafe { get_bytes(&self.program, &key.into()) };
        if val_ptr < 0 {
            return Err(StateError::Read);
        }

        // Wrap in OK for now, change from_raw_ptr to return Result
        unsafe { from_raw_ptr(val_ptr) }
    }
}

/// Converts a raw pointer to a deserialized value.
/// Expects the first 4 bytes of the pointer to represent the [length] of the serialized value,
/// with the subsequent [length] bytes comprising the serialized data.
/// # Panics
/// Panics if the bytes cannot be deserialized.
/// # Safety
/// This function is unsafe because it dereferences raw pointers.
/// # Errors
/// Returns an `StateError` if the bytes cannot be deserialized.
pub unsafe fn from_raw_ptr<V>(ptr: i64) -> Result<V, StateError>
where
    V: BorshDeserialize,
{
    let (bytes, _) = bytes_and_length(ptr);
    from_slice::<V>(&bytes).map_err(|_| StateError::Deserialization)
}

// TODO: move this logic to return a Memory struct that conatins ptr + length
/// Returns a tuple of the bytes and length of the argument.
/// # Panics
/// Panics if the value cannot be converted from i32 to usize.
/// # Safety
/// This function is unsafe because it dereferences raw pointers.
#[must_use]
pub unsafe fn bytes_and_length(ptr: i64) -> (Vec<u8>, usize) {
    type LenType = u32;

    let len = unsafe { std::slice::from_raw_parts(ptr as *const u8, 4) };

    assert_eq!(len.len(), std::mem::size_of::<LenType>());
    let len = LenType::from_be_bytes(len.try_into().unwrap()) as usize;

    let value = unsafe { std::slice::from_raw_parts(ptr as *const u8, len + 4) };
    (value[4..].to_vec(), len)
}

/// Returns a vector of bytes with the length of the argument prepended.
/// # Panics
/// Panics if the length of the argument cannot be converted to u32.
#[must_use]
pub fn prepend_length(bytes: &[u8]) -> Vec<u8> {
    let mut len_bytes = u32::try_from(bytes.len())
        .expect("pointer out range")
        .to_be_bytes()
        .to_vec();
    len_bytes.extend(bytes);
    len_bytes
}

/// Key is a wrapper around a Vec<u8> that represents a key in the host storage.
#[derive(Debug, Default, Clone)]
pub struct Key(Vec<u8>);

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
