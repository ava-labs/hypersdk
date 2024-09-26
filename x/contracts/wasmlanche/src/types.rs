// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

extern crate alloc;

use alloc::boxed::Box;
use borsh::{BorshDeserialize, BorshSerialize};
use bytemuck::{Pod, Zeroable};
use core::{array, mem::size_of};

/// Byte length of an action ID.
pub const ID_LEN: usize = 32;
/// Action id.
pub type Id = [u8; ID_LEN];

/// Gas type alias.
#[derive(Clone, Copy, Debug)]
pub enum Gas {
    PassAll,
    Units(u64),
}

impl From<u64> for Gas {
    fn from(value: u64) -> Self {
        Self::Units(value)
    }
}

impl BorshSerialize for Gas {
    fn serialize<W: borsh::io::Write>(&self, writer: &mut W) -> borsh::io::Result<()> {
        let units = match self {
            Self::PassAll => {
                true.serialize(writer)?;
                0
            }
            Self::Units(units) => {
                false.serialize(writer)?;
                *units
            }
        };
        units.serialize(writer)
    }
}

impl BorshDeserialize for Gas {
    fn deserialize_reader<R: borsh::io::Read>(reader: &mut R) -> borsh::io::Result<Self> {
        let pass_all = bool::deserialize_reader(reader)?;
        let units = u64::deserialize_reader(reader)?;
        let gas = if pass_all {
            if units > 0 {
                // return Err()
                todo!()
            }
            Gas::PassAll
        } else {
            Gas::Units(units)
        };
        Ok(gas)
    }
}

/// The ID bytes of a contract.
#[derive(BorshSerialize, BorshDeserialize)]
pub struct ContractId(Box<[u8]>);

#[cfg(feature = "test")]
impl From<Box<[u8]>> for ContractId {
    fn from(value: Box<[u8]>) -> Self {
        Self(value)
    }
}

/// Represents an address where a smart contract is deployed.
#[cfg_attr(feature = "debug", derive(Debug))]
#[derive(Clone, Copy, Ord, PartialOrd, PartialEq, Eq, BorshSerialize, BorshDeserialize, Hash)]
#[repr(transparent)]
pub struct Address([u8; 33]);

// # Safety: Pod is safe to implement for arrays of bytes
unsafe impl Zeroable for Address {}
unsafe impl Pod for Address {}

impl Address {
    pub const LEN: usize = size_of::<Self>();
    // Constructor function for Address
    #[must_use]
    pub fn new(bytes: [u8; Self::LEN]) -> Self {
        Self(bytes)
    }
}

impl Default for Address {
    fn default() -> Self {
        Self([0; Self::LEN])
    }
}

impl IntoIterator for Address {
    type Item = u8;
    type IntoIter = array::IntoIter<Self::Item, { Address::LEN }>;

    fn into_iter(self) -> Self::IntoIter {
        IntoIterator::into_iter(self.0)
    }
}

impl AsRef<[u8]> for Address {
    fn as_ref(&self) -> &[u8] {
        &self.0
    }
}
