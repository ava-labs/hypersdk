use borsh::{BorshDeserialize, BorshSerialize};

/// Byte length of an action ID.
pub const ID_LEN: usize = 32;
/// Action id.
pub type Id = [u8; ID_LEN];
/// Gas type alias.
pub type Gas = u64;

/// A struct that enforces a fixed length of 32 bytes which represents an address.
#[cfg_attr(feature = "debug", derive(Debug))]
#[derive(Clone, Copy, PartialEq, Eq, BorshSerialize, BorshDeserialize, Hash)]
pub struct Address([u8; Self::LEN]);

impl Address {
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

    #[allow(clippy::should_implement_trait)]
    #[cfg(feature = "test-utils")]
    #[must_use]
    /// # Panics
    /// Panics if the length of the bytes representation of the name is longer than [`Address::LEN`]
    pub fn from_str(name: &str) -> Self {
        let mut bytes = [0u8; Self::LEN];
        let name_bytes = name.as_bytes();
        assert!(name_bytes.len() <= Self::LEN, "passed name is too long");
        bytes[0..name_bytes.len()].copy_from_slice(name_bytes);
        Self::new(bytes)
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
