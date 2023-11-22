use std::borrow::Cow;
use borsh::{BorshDeserialize, BorshSerialize};
use crate::program::Program;

pub const ADDRESS_LEN: usize = 32;
/// A struct that enforces a fixed length of 32 bytes which represents an address.

#[derive(Clone, Copy, PartialEq, Eq, Debug, BorshSerialize, BorshDeserialize)]
pub struct Address([u8; Self::LEN]);


impl Address {
    pub const LEN: usize = 32;
    // Constructor function for Address
    #[must_use]
    pub fn new(bytes: [u8; ADDRESS_LEN]) -> Self {
        Self(bytes)
    }
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl IntoIterator for Address {
    type Item = u8;
    type IntoIter = std::array::IntoIter<Self::Item, ADDRESS_LEN>;

    fn into_iter(self) -> Self::IntoIter {
        IntoIterator::into_iter(self.0)
    }
}

pub fn is_primitive<T>() -> bool {
    // Check if the type is a supported primitive (i64 or i32 or bool)
    std::mem::size_of::<T>() == std::mem::size_of::<i64>() || std::mem::size_of::<T>() == std::mem::size_of::<i32>() || std::mem::size_of::<T>() == std::mem::size_of::<bool>()
}