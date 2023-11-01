use serde::{de::DeserializeOwned, Serialize};
use serde_bare::to_vec;

use crate::{
    errors::StateError,
    host::{delete_bytes, get_bytes, put_bytes},
    memory::Memory,
    program::Program,
    types::Bytes32,
};

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

impl From<&str> for Key {
    fn from(s: &str) -> Self {
        // Convert the string slice into a byte slice
        let bytes = s.as_bytes();

        Self(bytes.to_vec())
    }
}

impl From<Key> for i64 {
    fn from(key: Key) -> Self {
        let bytes = key.0;
        let mut arr = [0u8; 8];
        arr.copy_from_slice(&bytes[0..8]);
        i64::from_be_bytes(arr)
    }
}

impl From<i64> for Key {
    fn from(item: i64) -> Self {
        let bytes = item.to_be_bytes();
        Self(bytes.to_vec())
    }
}

#[derive(Debug, Default, Clone)]
pub struct Value{
    bytes: [u8; 8],
}

impl Value {
    #[must_use]
    pub fn new(bytes:[u8; 8]) -> Self {
        Self{bytes}
    }

    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.bytes
    }

    #[must_use]
    pub fn len(&self) -> usize {
        self.bytes.len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

struct StoredValue<const N: usize> {
    type_prefix: u8,
    // key: Key // <- state macro would have to implement an infallible conversion from user-defined StateKey to Key
    value: [u8; N],
}

impl From<&str> for Value {
    fn from(s: &str) -> Self {
        // Convert the string slice into a byte slice
        let bytes = s.as_bytes();

        Value(bytes.to_vec())
    }
}

impl From<Value> for i64 {
    fn from(value: Value) -> Self {
        println!("from value");
        let bytes = value.as_bytes();
        let mut arr = [0u8; 8];
        arr.copy_from_slice(&bytes[0..8]);
        i64::from_be_bytes(arr)
    }
}

impl From<i64> for Value {
    fn from(item: i64) -> Self {
        let bytes = item.to_be_bytes();

        println!("from i64");
        Value(bytes.to_vec())
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
    pub fn store<K, V>(&self, key: K, value: V) -> Result<(), StateError>
    where
        K: Into<Key>,
        V: Into<Value>,
    {
        put_bytes(&self.program, key.into(), value.into())
    }

    /// Get a value from the host's storage.
    ///
    /// Note: The pointer passed to the host are only valid for the duration of this
    /// function call. This function will take ownership of the pointer and free it.
    ///
    /// # Errors
    /// Returns an `StateError` if the key cannot be serialized or if
    /// the host fails to read the key and value.
    pub fn get<K>(&self, key: K) -> Result<Value, StateError>
    where
        K: Into<Key>,
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

pub struct Storable {
    key: Key,
    value: Value,
}

// impl Storable {
//     pub fn new(key: Key, value: Value) -> Self {
//         Self { key, value }
//     }

//     // bucket 0 -> singleton
//     // bucket 1 -> map
//     // bucket 2 -> vector
//     // bucket 3 -> arra
//     // [bucket,prefix, key]
//     pub fn key(&self) -> Key {
//         self.key
//     }

//     pub fn value(&self) -> Value {
//         self.value
//     }
// }

// impl Key {
//     pub fn new(bytes: [u8; M]) -> Self {
//         Self(bytes)
//     }
//     pub fn from_bytes(bytes: [u8; M]) -> Self {
//         Key(bytes)
//     }
// }

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

// impl<const N: usize> Value<N> {
//     fn from_bytes(bytes: [u8; N]) -> Self {
//         Value(bytes)
//     }
// }
