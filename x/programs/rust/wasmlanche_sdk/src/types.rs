use serde::{Deserialize, Serialize};
use std::borrow::Cow;

use crate::{program::Program, state::bytes_and_length};

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

// pub struct VecArg<T>(Vec<T>);
// // TODO: this should be size & not length
// pub trait HasLen {
//     fn len(&self) -> usize;
// }
// impl<T> From<i64> for VecArg<T> 
// where T: From<i64> {
//     fn from(value: i64) -> Self {
//         let (bytes, len) = bytes_and_length(value);
//         let len = len as usize;
//         let mut vec = Vec::new();

//         vec.push(value.into());
//         Self(Bytes32::from(value))
//     }
// }


/// Implement the Display trait for Bytes32 so that we can print it.
/// Enables `to_string()` on Bytes32.
impl std::fmt::Display for Bytes32 {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
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

pub trait Argument {
    fn as_bytes(&self) -> Cow<'_, [u8]>;
    fn from_bytes(bytes: &[u8]) -> Self
    where
        Self: Sized;
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

impl Argument for Bytes32 {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(&self.0)
    }
    fn from_bytes(bytes: &[u8]) -> Self
        where
            Self: Sized {
        let bytes = bytes[..ADDRESS_LEN].try_into().unwrap();
        Self::new(bytes)
    }
}

impl Argument for Address {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Borrowed(self.0.as_bytes())
    }
    fn from_bytes(bytes: &[u8]) -> Self {
        // get first ADDRESS_LEN bytes from bytes
        let bytes = bytes[..ADDRESS_LEN].try_into().unwrap();
        Self::new(bytes)
    }
}

impl Argument for i64 {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Owned(self.to_be_bytes().to_vec())
    }
    fn is_primitive(&self) -> bool {
        true
    }
    fn from_bytes(bytes: &[u8]) -> Self {
        let bytes = bytes[..8].try_into().unwrap();
        Self::from_be_bytes(bytes)
    }
}


impl Argument for i32 {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Owned(self.to_be_bytes().to_vec())
    }
    fn is_primitive(&self) -> bool {
        true
    }
    fn from_bytes(bytes: &[u8]) -> Self {
        let bytes = bytes[..4].try_into().unwrap();
        Self::from_be_bytes(bytes)
    }
}

impl Argument for Program {
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        Cow::Owned(self.id().to_be_bytes().to_vec())
    }
    fn is_primitive(&self) -> bool {
        true
    }
    fn from_bytes(bytes: &[u8]) -> Self {
        let id : i64 = i64::from_bytes(bytes);
        Self::from(id)
    }
}

pub struct VecArg<T>(Vec<T>);

impl<T> VecArg<T> {
    #[must_use]
    pub fn new(vec: Vec<T>) -> Self {
        Self(vec)
    }
    pub fn as_vec(&self) -> &Vec<T> {
        &self.0
    }
    pub fn len(&self) -> usize {
        self.0.len()
    }
}

impl<T> Argument for VecArg<T>
where
    T: Argument,
{
    // we don't know how large each element T is, but we know that each element has a from_bytes method
    fn from_bytes(bytes: &[u8]) -> Self {
        // Vec to be returned
        let mut vec = Vec::new();
        let mut current_byte = 0;
        let num_bytes = bytes.len();
        // TODO: check logic on empty vec
        while current_byte < num_bytes {
            // copy the bytes into a new vec
            let elem : T = T::from_bytes(&bytes[current_byte..]);
            current_byte += elem.len();
            vec.push(elem);
        }

        Self(vec)
    }
    fn as_bytes(&self) -> Cow<'_, [u8]> {
        let mut bytes = Vec::new();
        for elem in &self.0 {
            bytes.extend_from_slice(&elem.as_bytes());
        }
        Cow::Owned(bytes)
    }
}