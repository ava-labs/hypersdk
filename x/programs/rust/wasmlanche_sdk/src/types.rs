use serde::{
    de::{self, SeqAccess, Visitor},
    Deserialize, Deserializer, Serialize, Serializer,
};
use std::{borrow::Cow, fmt};

use crate::program::Program;

pub const ADDRESS_LEN: usize = 33;
/// A struct that enforces a fixed length of 32 bytes which represents an address.
#[derive(Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Debug)]
pub struct Address(Bytes<ADDRESS_LEN>);

impl Address {
    // Constructor function for Address
    #[must_use]
    pub fn new(bytes: [u8; ADDRESS_LEN]) -> Self {
        Self(Bytes::new(bytes))
    }
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        self.0.as_bytes()
    }
}

impl IntoIterator for Address {
    type Item = u8;
    type IntoIter = std::array::IntoIter<Self::Item, ADDRESS_LEN>;

    fn into_iter(self) -> Self::IntoIter {
        IntoIterator::into_iter(self.0 .0)
    }
}

/// A struct representing variable length bytes.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct Bytes<const N: usize>([u8; N]);

impl<const N: usize> Serialize for Bytes<N> {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        // Serialize the array as a sequence of bytes
        serializer.serialize_bytes(&self.0)
    }
}

impl<'de, const N: usize> Deserialize<'de> for Bytes<N> {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        struct BytesVisitor<const N: usize>;

        impl<'de, const N: usize> Visitor<'de> for BytesVisitor<N> {
            type Value = Bytes<N>;

            fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
                formatter.write_str(&format!("{N} bytes"))
            }

            fn visit_seq<A>(self, mut seq: A) -> Result<Bytes<N>, A::Error>
            where
                A: SeqAccess<'de>,
            {
                let mut bytes = [0u8; N];
                for (i, byte) in bytes.iter_mut().enumerate() {
                    *byte = seq.next_element::<u8>()?.ok_or_else(|| {
                        de::Error::invalid_length(i, &format!("{N} bytes").as_str())
                    })?;
                }
                Ok(Bytes(bytes))
            }
        }

        deserializer.deserialize_bytes(BytesVisitor::<N>)
    }
}

impl<const N: usize> Bytes<N> {
    #[must_use]
    pub fn new(bytes: [u8; N]) -> Self {
        Self(bytes)
    }
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl From<i64> for Bytes<32> {
    fn from(value: i64) -> Self {
        let bytes: [u8; 32] = unsafe {
            // We want to copy the bytes here, since [value] represents a ptr created by the host
            std::slice::from_raw_parts(value as *const u8, 32)
                .try_into()
                .unwrap()
        };
        Self(bytes)
    }
}

pub trait Argument {
    fn as_bytes(&self) -> Cow<'_, [u8]>;
    fn is_primitive(&self) -> bool {
        false
    }
    fn len(&self) -> usize {
        self.as_bytes().len()
    }
    fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

impl<const N: usize> Argument for Bytes<N> {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(&self.0)
    }
}

impl Argument for Address {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(self.0.as_bytes())
    }
}

impl Argument for i64 {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Owned(self.to_be_bytes().to_vec())
    }
    fn is_primitive(&self) -> bool {
        true
    }
}

impl Argument for Program {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Owned(self.id().to_be_bytes().to_vec())
    }
    fn is_primitive(&self) -> bool {
        true
    }
}
