use byteorder::{BigEndian, ByteOrder};
use serde::{de::DeserializeOwned, Serialize};
use serde_bare::to_vec;

use crate::{
    errors::StateError,
    host::{delete_bytes, get_bytes, put_bytes},
    memory::Memory,
    program::Program,
    types::{Bytes32, TypePrefix},
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

/// Represents an i64 passed from the host to the program. The first 4 bytes represents the length and the last 4 bytes represent the pointer to the value.
pub struct HostValue {
    bytes: Vec<u8>,
}

#[derive(Debug, Clone)]
pub struct Value {
    type_prefix: TypePrefix,
    bytes: Vec<u8>,
}
//    1    4
// [type][len][bytes]
impl Value {
    #[must_use]
    pub fn new(type_prefix: TypePrefix, bytes: Vec<u8>) -> Self {
        Self { type_prefix, bytes }
    }

    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        // fix me
        &self.bytes
    }

    #[must_use]
    pub fn len(&self) -> usize {
        // fix me the total len would include the type prefix etc
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

        // TODO: method that returns the type from s

        Value{ type_prefix:TypePrefix::Bytes , bytes: bytes.to_vec() }
    }
}

//
impl From<Value> for i64 {
    fn from(value: Value) -> Self {
        println!("from value");
        let bytes = value.as_bytes();
        let mut arr = [0u8; 8];
        arr.copy_from_slice(&bytes[0..8]);
        i64::from_be_bytes(arr)
    }
}

/// converts an i64 from the host to the value
impl From<i64> for Value {
    fn from(value: i64) -> Self {
        let value_bytes = value.to_be_bytes();

        // extract lower 4 bytes as length
        let len = usize::try_from(BigEndian::read_u32(&value_bytes[0..4]))
            .expect("failed to convert size to usize");
        // extract upper 4 bytes as pointer
        let ptr = BigEndian::read_u32(&value_bytes[4..8]);

        let bytes = unsafe {
            std::slice::from_raw_parts(ptr as *const u8, len)
                .try_into()
                .expect("failed create byte slice from ptr")
        };

        Value::new(bytes)
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
    pub fn get<K>(&self, key: K) -> Result<Vec<u8>, StateError>
    where
        K: Into<Key>,
    {
        get_bytes(&self.program, key.into())
    }
}

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


/// On disk state is linear. State root is segmented into two options: a singleton key and a key bucket. In complex data structures such as for example a map you can use a bucket and struct to describe this data. The bucket is a 64 byte array which is used to store the key-value pairs. The first 4 bytes of the bucket is used to store the number of key-value pairs in the bucket. The remaining 60 bytes are used to store the key-value pairs. The key-value pairs are stored sequentially in the bucket. The
/// 
/// On disk we would perform a range
/// hashed_name, [metadata], "name"
/// [bucket_key, prefix_type, key]
/// [bucket_key, prefix_type, key]

type Bucket = [u8; 64];

struct Metadata {
    /// The number of key-value pairs in the bucket.
    count: u32,
    /// The number of bytes used to store the key-value pairs.
    used_bytes: u32,
}



enum StateKeyRoot {
    /// Represents a single key which would be used for a key-value pair.
    SingletonKey, // 0x0
    BucketetKey(Bucket, Metadata), // 0x1
}