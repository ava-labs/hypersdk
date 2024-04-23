//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

use crate::state::Error as StateError;
use borsh::{from_slice, BorshDeserialize};
use std::collections::HashMap;
use std::mem::ManuallyDrop;
use std::{alloc::Layout, cell::RefCell, collections::HashSet};

/// Represents a pointer to a block of memory allocated by the global allocator.
#[derive(Clone, Copy)]
pub struct Pointer(*mut u8);

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

/// `HostPtr` is an i64 where the first 4 bytes represent the length of the bytes
/// and the following 4 bytes represent a pointer to WASM memeory where the bytes are stored.
pub type HostPtr = i64;

thread_local! {
    static GLOBAL_STORE: RefCell<HashMap<*const u8, usize>> = RefCell::new(HashMap::new());
}

/// Converts a pointer to a i64 with the first 4 bytes of the pointer
/// representing the length of the memory block.
/// # Errors
/// Returns an [`StateError`] if the pointer or length of `args` exceeds
/// the maximum size of a u32.
#[allow(clippy::cast_possible_truncation)]
pub fn to_host_ptr(arg: &[u8]) -> Result<HostPtr, StateError> {
    let ptr = arg.as_ptr() as usize;
    let len = arg.len();

    // Make sure the pointer and length fit into u32
    if ptr > u32::MAX as usize || len > u32::MAX as usize {
        return Err(StateError::IntegerConversion);
    }

    let host_ptr = i64::from(ptr as u32) | (i64::from(len as u32) << 32);
    Ok(host_ptr)
}

/// Converts a i64 to a pointer with the first 4 bytes of the pointer
/// representing the length of the memory block.
/// # Panics
/// Panics if arg is negative.
#[must_use]
#[allow(clippy::cast_sign_loss)]
pub fn split_host_ptr(arg: HostPtr) -> (i64, usize) {
    assert!(arg >= 0);

    let len = arg >> 32;
    let mask: u32 = !0;
    let ptr = arg & i64::from(mask);
    (ptr, len as usize)
}

/// Converts a raw pointer to a deserialized value.
/// Expects the first 4 bytes of the pointer to represent the `length` of the serialized value,
/// with the subsequent `length` bytes comprising the serialized data.
/// # Panics
/// Panics if the bytes cannot be deserialized.
/// # Safety
/// This function is unsafe because it dereferences raw pointers.
/// # Errors
/// Returns an [`StateError`] if the bytes cannot be deserialized.
pub fn from_host_ptr<V>(ptr: HostPtr) -> Result<V, StateError>
where
    V: BorshDeserialize,
{
    let (ptr, len) = split_host_ptr(ptr);
    match into_bytes(ptr) {
        Some(bytes) => from_slice::<V>(&bytes).map_err(|_| StateError::Deserialization),
        None => Err(StateError::InvalidBytes),
    }
}

/// Returns a tuple of the bytes and length of the argument.
/// `host_ptr` is encoded using Big Endian as an i64.
#[must_use]
fn into_bytes(ptr: i64) -> Option<Vec<u8>> {
    // grab length from ptrArg
    if let Some(len) = GLOBAL_STORE.with_borrow_mut(|s| s.remove(&(ptr as *const u8))) {
        let ptr = ptr as *mut u8;
        Some(unsafe { std::vec::Vec::from_raw_parts(ptr, len as usize, len as usize) })
    } else {
        None
    }
}

/* memory functions ------------------------------------------- */
/// Allocate memory into the instance of Program and return the offset to the
/// start of the block.
/// # Panics
/// Panics if the pointer exceeds the maximum size of an i64.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> *mut u8 {
    assert!(len > 0, "cannot allocate 0 sized data");
    // from https://doc.rust-lang.org/1.77.2/src/alloc/raw_vec.rs.html#547-562
    let layout = Layout::array::<u8>(len).expect("capacity overflow");
    // take a mutable pointer to the layout
    let ptr = unsafe { std::alloc::alloc(layout) };
    // keep track of the pointer
    // ManuallyDrop::drop(ptr);
    GLOBAL_STORE.with_borrow_mut(|s| s.insert(ptr, len));
    // return the pointer so the runtime
    // can write data at this offset
    ptr
}

/// # Safety
/// `ptr` must be a pointer to a block of memory.
///
/// deallocates the memory block at `ptr` with a given `capacity`.
#[no_mangle]
pub unsafe extern "C" fn dealloc(ptr: *mut u8, capacity: usize) {
    // always deallocate the full capacity, initialize vs uninitialized memory is irrelevant here
    let data = Vec::from_raw_parts(ptr, capacity, capacity);
    std::mem::drop(data);
}

#[cfg(test)]
mod tests {
    use super::{alloc, into_bytes};
    use crate::memory::GLOBAL_STORE;

    #[test]
    fn data_allocation() {
        let len = 1024;
        let ptr = alloc(len);
        let vec = vec![1; len];
        vec.iter().enumerate().for_each(|(i, val)| unsafe {
            *ptr.add(i) = *val;
        });
        let val = into_bytes(ptr as i64).unwrap();
        assert_eq!(val, vec);
        assert!(GLOBAL_STORE.with_borrow(|s| s.get(&(ptr as *const u8)).is_none()));
    }
}
