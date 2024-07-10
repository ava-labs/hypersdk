use borsh::BorshDeserialize;
#[cfg(feature = "test-utils")]
use serde::Serialize;

/// Byte length of an action ID.
pub const ID_LEN: usize = 32;
/// Action id.
pub type Id = [u8; ID_LEN];
/// Gas type alias.
pub type Gas = u64;

/// A struct that enforces a fixed length of 32 bytes which represents an address.
#[cfg_attr(feature = "debug", derive(Debug))]
#[cfg_attr(feature = "test-utils", derive(Serialize))]
#[derive(Clone, Copy, PartialEq, Eq, borsh::BorshSerialize, BorshDeserialize, Hash)]
pub struct Address(
    #[cfg_attr(feature = "test-utils", serde(serialize_with = "<[_]>::serialize"))] [u8; Self::LEN],
);

impl Address {
    // TODO: move to HyperSDK.Address which will be 33 bytes
    pub const LEN: usize = 33;
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

impl Default for Address {
    fn default() -> Self {
        Self([0; Self::LEN])
    }
}

impl IntoIterator for Address {
    type Item = u8;
    type IntoIter = std::array::IntoIter<Self::Item, { Address::LEN }>;

    fn into_iter(self) -> Self::IntoIter {
        IntoIterator::into_iter(self.0)
    }
}

impl AsRef<[u8]> for Address {
    fn as_ref(&self) -> &[u8] {
        &self.0
    }
}
