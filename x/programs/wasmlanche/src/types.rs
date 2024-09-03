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
pub type Gas = u64;

/// The ID bytes of a program.
#[derive(BorshSerialize, BorshDeserialize, Clone)]
pub struct ProgramId(Box<[u8]>);

impl From<Box<[u8]>> for ProgramId {
    fn from(value: Box<[u8]>) -> Self {
        Self(value)
    }
}

#[cfg(feature = "unit_tests")]
impl ProgramId {
    #[must_use]
    pub fn new(bytes: Box<[u8]>) -> Self {
        Self(bytes)
    }
}

/// Represents an address where a smart contract is deployed.
#[cfg_attr(feature = "debug", derive(Debug))]
#[derive(Clone, Copy, PartialEq, Eq, BorshSerialize, BorshDeserialize, Hash)]
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
