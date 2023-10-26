use serde::{Deserialize, Serialize};
use std::borrow::Cow;

use crate::{memory::Memory, program::Program};

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

/// Represents a pointer to a block of memory allocated by the global allocator.
/// A `Pointer` can be used directly as a param type on a `public` functions.
///
/// # Example
/// ```
/// use wasmlanche_sdk::types::Pointer;
/// #[public]
/// pub fn init( program: Program, name_ptr: Pointer, name_length: i64) -> bool {
/// let nft_name = name_ptr.read(name_length as usize);
/// ```
///
#[derive(Clone, Copy)]
pub struct Pointer(*mut u8);

impl Pointer {
    pub fn new(ptr: *mut u8) -> Self {
        Self(ptr)
    }

    #[must_use]
    pub fn inner(&self) -> *mut u8 {
        self.0
    }

    #[must_use]
    /// Attempts return a copy of the bytes from a pointer created by the global allocator.
    /// # Safety
    /// `ptr` must be a pointer to a block of memory created using alloc.
    /// `length` must be the length of the block of memory.
    pub unsafe fn read(self, len: usize) -> Vec<u8> {
        unsafe { Memory::new(self).range(len) }
    }
}

impl From<i64> for Pointer {
    fn from(v: i64) -> Self {
        let ptr: *mut u8 = v as *mut u8;
        Pointer(ptr)
    }
}

impl From<Pointer> for *const u8 {
    fn from(pointer: Pointer) -> Self {
        pointer.0.cast_const()
    }
}

impl From<Pointer> for *mut u8 {
    fn from(pointer: Pointer) -> Self {
        pointer.0
    }
}
