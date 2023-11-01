use serde::{de::DeserializeOwned, Serialize};
use serde_bare::to_vec;

use crate::{
    errors::StateError,
    host::{delete_bytes, get_bytes, put_bytes},
    memory::Memory,
    program::Program,
};

#[derive(Debug, Copy, Clone)]
pub struct Key<const N: usize>([u8; N]);

impl<const N: usize> From<Key<N>> for i64
where
    [(); 8]: Sized,
{
    fn from(key: Key<N>) -> Self {
        let mut arr = [0u8; 8];
        arr[..N].copy_from_slice(&key.0[..N]);
        i64::from_be_bytes(arr)
    }
}

#[derive(Debug, Copy, Clone)]
pub struct Value<const M: usize>([u8; M]);

impl<const M: usize> Default for Value<M> {
    fn default() -> Self {
        Self([0u8; M])
    }
}

impl<const M: usize> From<Value<M>> for i64
where
    [(); 8]: Sized,
{
    fn from(key: Value<M>) -> Self {
        let mut arr = [0u8; 8];
        arr[..M].copy_from_slice(&key.0[..M]);
        i64::from_be_bytes(arr)
    }
}

impl<const M: usize> From<i64> for Value<M>
where
    [(); 8]: Sized,
{
    fn from(val: i64) -> Self {
        let bytes = val.to_be_bytes();
        let mut arr = [0u8; M];
        arr[..8].copy_from_slice(&bytes);
        Value(arr)
    }
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
    /// Returns an `StateError` if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<K, V, const M: usize, const N: usize>(
        &self,
        key: K,
        value: V,
    ) -> Result<(), StateError>
    where
        K: Into<Key<M>>,
        V: Into<Value<N>>,
    {
        let storable = Storable {
            key: key.into(),
            value: value.into(),
        };

        put_bytes(&self.program, storable)
    }

    /// Get a value from the host's storage.
    ///
    /// Note: The pointer passed to the host are only valid for the duration of this
    /// function call. This function will take ownership of the pointer and free it.
    ///
    /// # Errors
    /// Returns an `StateError` if the key cannot be serialized or if
    /// the host fails to read the key and value.
    pub fn get<K, const M: usize, const N: usize>(&self, key: K) -> Result<Value<N>, StateError>
    where
        K: Into<Key<M>>,
    {
        get_bytes(&self.program, key.into())
    }
}

// fn from_slice<K, const M: usize, const N: usize>(slice: &[u8]) -> Result<Storable<M,N>, StateError> {
//     // We need at least 1 byte for the type_prefix, plus the size of the key, plus N bytes for the value.
//     if slice.len() < 1 + std::mem::size_of::<Key<N>>() + N {
//         return Err(StateError::InvalidBytes);
//     }

//     let value_type_prefix = slice[0];

//     // For simplicity, let's assume Key is of fixed size, say 8 bytes.
//     let key_bytes = &slice[1..1 + std::mem::size_of::<Key<N>>()];
//     let key = Key::from_bytes(key_bytes.try_into().expect("Incorrect key size"));

//     let value_bytes = &slice[1 + std::mem::size_of::<Key<N>>()..];
//     let mut value = [0u8; N];
//     value.copy_from_slice(value_bytes);

//     Ok(Storable {
//         key,
//         value,
//     })
// }

pub struct Storable<const M: usize, const N: usize> {
    key: Key<M>,
    value: Value<N>,
}

impl<const M: usize, const N: usize> Storable<M, N> {
    pub fn new(key: Key<M>, value: Value<N>) -> Self {
        Self { key, value }
    }

    // bucket 0 -> singleton
    // bucket 1 -> map
    // bucket 2 -> vector
    // bucket 3 -> arra
    // [bucket,prefix, key]
    pub fn key(&self) -> Key<M> {
        self.key
    }

    pub fn value(&self) -> Value<N> {
        self.value
    }
}

impl<const M: usize> Key<M> {
    pub fn new(bytes: [u8; M]) -> Self {
        Self(bytes)
    }
    pub fn from_bytes(bytes: [u8; M]) -> Self {
        Key(bytes)
    }
}

// impl<const M: usize, const N: usize> From<Key<M>> for Value<N> {
//     fn from(key: Key<M>) -> Self {
//         let bytes: [u8; Self::LEN] = unsafe {
//             let storable = Storable::new(key, Value::default());

//             let mut bytes = Vec::new();

//             // We want to copy the bytes here, since [value] represents a ptr created by the host
//             std::slice::from_raw_parts(key.0.as_ptr(), N)
//                 .try_into()
//                 .unwrap()
//         };
//         Self(bytes)
//     }
// }

impl<const N: usize> Value<N> {
    fn from_bytes(bytes: [u8; N]) -> Self {
        Value(bytes)
    }
}
