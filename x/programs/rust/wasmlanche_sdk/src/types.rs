use serde::{Deserialize, Serialize};
use std::{
    borrow::Cow,
    fmt::{Display, Formatter},
};

use crate::program::Program;

pub const ADDRESS_LEN: usize = 32;
/// A struct that enforces a fixed length of 32 bytes which represents an address.
#[derive(Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Debug)]
pub struct Address(Bytes32);

impl Address {
    // Constructor function for Address
    #[must_use]
    pub fn new(bytes: [u8; ADDRESS_LEN]) -> Self {
        Self(Bytes32::new(bytes))
    }
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        self.0.as_bytes()
    }
}

impl From<i64> for Address {
    fn from(value: i64) -> Self {
        Self(Bytes32::from(value))
    }
}

impl IntoIterator for Address {
    type Item = u8;
    type IntoIter = std::array::IntoIter<Self::Item, ADDRESS_LEN>;

    fn into_iter(self) -> Self::IntoIter {
        IntoIterator::into_iter(self.0 .0)
    }
}

/// A struct representing a fixed length of 32 bytes.
/// This can be used for passing strings to the host. It caps the string at 32 bytes,
#[derive(Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Debug)]
pub struct Bytes32([u8; Self::LEN]);
impl Bytes32 {
    pub const LEN: usize = 32;
    #[must_use]
    pub fn new(bytes: [u8; Self::LEN]) -> Self {
        Self(bytes)
    }
    #[must_use]
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

/// Implement the Display trait for Bytes32 so that we can print it.
/// Enables `to_string()` on Bytes32.
impl Display for Bytes32 {
    fn fmt(&self, f: &mut Formatter) -> std::fmt::Result {
        // Find the first null byte and only print up to that point.
        let null_pos = self.0.iter().position(|&b| b == b'\0').unwrap_or(Self::LEN);
        String::from_utf8_lossy(&self.0[..null_pos]).fmt(f)
    }
}

impl From<String> for Bytes32 {
    fn from(value: String) -> Self {
        let mut bytes: [u8; Self::LEN] = [0; Self::LEN];
        bytes[..value.len()].copy_from_slice(value.as_bytes());
        Self(bytes)
    }
}

impl From<i64> for Bytes32 {
    fn from(value: i64) -> Self {
        let bytes: [u8; Self::LEN] = unsafe {
            // We want to copy the bytes here, since [value] represents a ptr created by the host
            std::slice::from_raw_parts(value as *const u8, Self::LEN)
                .try_into()
                .unwrap()
        };
        Self(bytes)
    }
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct Bytes<const N: usize>([u8; N]);

impl<const N: usize> Bytes<N> {
    pub fn new(bytes: [u8; N]) -> Self {
        Self(bytes)
    }
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl<const N: usize> From<String> for Bytes<N> {
    fn from(value: String) -> Self {
        let mut bytes: [u8; N] = [0; N];
        bytes[..value.len()].copy_from_slice(value.as_bytes());
        Self(bytes)
    }
}

impl<const N: usize> Display for Bytes<N> {
    fn fmt(&self, f: &mut Formatter) -> std::fmt::Result {
        // Find the first null byte and only print up to that point.
        let null_pos = self.0.iter().position(|&b| b == b'\0').unwrap_or(N);
        String::from_utf8_lossy(&self.0[..null_pos]).fmt(f)
    }
}

// pub trait Arg<const N: usize> {
//     fn as_bytes(&self) -> [u8; N];
//     /// Returns the prefix byte for the argument.
//     fn prefix(&self) -> u8;
//     /// Returns the length of the argument in bytes.
//     fn len(&self) -> usize {
//         self.as_bytes().len()
//     }
//     fn is_empty(&self) -> bool {
//         self.len() == 0
//     }
// }

#[repr(u8)]
#[derive(Clone, Copy, Serialize, Deserialize)]
pub enum ArgTypes {
    I64 = 0x0,
    I32,
    Bytes32,
    Bytes,
}

pub enum Arg {
    I64(i64),
    Bytes32(Bytes32),
    Address(Address),
    Bytes(Vec<u8>),
}

impl Arg {
    pub fn prefix(&self) -> u8 {
        match self {
            Arg::I64(_) => ArgTypes::I64 as u8,
            Arg::Bytes32(_) => ArgTypes::Bytes32 as u8,
            Arg::Address(_) => ArgTypes::Bytes32 as u8,
            Arg::Bytes(_) => ArgTypes::Bytes as u8,
        }
    }
    pub fn as_bytes(&self) -> Vec<u8> {
        match self {
            Arg::I64(i) => i.to_be_bytes().to_vec(),
            Arg::Bytes32(b) => b.as_bytes().to_vec(),
            Arg::Address(a) => a.as_bytes().to_vec(),
            Arg::Bytes(b) => b.as_slice().to_vec(),
        }
    }

    pub fn len(&self) -> usize {
        self.as_bytes().len()
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

impl From<i64> for Arg {
    fn from(value: i64) -> Self {
        Self::I64(value)
    }
}

impl From<Bytes32> for Arg {
    fn from(value: Bytes32) -> Self {
        Self::Bytes32(value)
    }
}

impl From<Address> for Arg {
    fn from(value: Address) -> Self {
        Self::Address(value)
    }
}

// impl Arg<32> for Bytes32 {
//     fn as_bytes(&self) -> [u8; 32] {
//         self.0
//     }
//     fn prefix(&self) -> u8 {
//         ArgTypes::Bytes32 as u8
//     }
// }

// impl <const N: usize>Arg<N> for Bytes<N> {
//     fn as_bytes(&self) -> [u8; N] {
//         self.0
//     }
//     fn prefix(&self) -> u8 {
//         ArgTypes::Bytes as u8
//     }
// }

// impl Arg<32> for Address {
//     fn as_bytes(&self) ->  [u8; 32] {
//         self.0.as_bytes()
//     }
//     fn prefix(&self) -> u8 {
//         ArgTypes::Bytes32 as u8
//     }
// }

// impl Arg<8> for i64 {
//     fn as_bytes(&self) -> [u8; 8] {
//         self.to_be_bytes()
//     }
//     fn prefix(&self) -> u8 {
//         ArgTypes::I64 as u8
//     }
// }

// impl Arg<32> for Program {
//     fn as_bytes(&self) -> [u8; 32] {
//         self.id().to_be_bytes()
//     }
//     fn prefix(&self) -> u8 {
//         ArgTypes::I64 as u8
//     }
// }

// #[cfg(test)]
// mod tests {
//     use super::*;

//     #[test]
//     fn test_bytes32_from_string() {
//         let s = "hello world".to_string();
//         let b = Bytes32::from(s);
//         assert_eq!(b.to_string(), "hello world");
//     }

//     #[test]
//     fn test_bytes32_from_i64() {
//         let i = 123456789;
//         let b = Bytes32::from(i);
//         assert_eq!(b.to_string(), "123456789");
//     }

//     #[test]
//     fn test_bytes_from_address() {
//         let s = "hello world".to_string();
//         let b = Bytes::from(s);
//         assert_eq!(b.to_string(), "hello world");
//     }
// }
