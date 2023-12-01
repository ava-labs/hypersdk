use crate::program::Program;
use borsh::{BorshDeserialize, BorshSerialize};
use std::borrow::Cow;

/// A struct that enforces a fixed length of 32 bytes which represents an address.

#[derive(Clone, Copy, PartialEq, Eq, Debug, BorshSerialize, BorshDeserialize)]
pub struct Address([u8; Self::LEN]);

impl Address {
    // TODO: move to HyperSDK.Address which will be 33 bytes
    pub const LEN: usize = 32;
    // Constructor function for Address
    #[must_use]
    pub fn new(bytes: [u8; Self::LEN]) -> Self {
        Self(bytes)
    }
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl IntoIterator for Address {
    type Item = u8;
    type IntoIter = std::array::IntoIter<Self::Item, { Address::LEN }>;

    fn into_iter(self) -> Self::IntoIter {
        IntoIterator::into_iter(self.0)
    }
}

/// A trait that represents an argument that can be passed to & from the host.
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

impl Argument for Address {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(self.as_bytes())
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

impl Argument for i32 {
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

impl Argument for String {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(self.as_bytes())
    }
}
