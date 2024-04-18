use borsh::{BorshDeserialize, BorshSerialize};

/// A struct that enforces a fixed length of 32 bytes which represents an address.

#[derive(Clone, Copy, PartialEq, Eq, Debug, BorshSerialize, BorshDeserialize, Hash)]
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
