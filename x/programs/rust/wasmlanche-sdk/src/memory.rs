//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

use crate::state::Error as StateError;
use borsh::{from_slice, BorshDeserialize};

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
pub unsafe fn from_host_ptr<V>(ptr: HostPtr) -> Result<V, StateError>
where
    V: BorshDeserialize,
{
    let bytes = into_bytes(ptr);
    from_slice::<V>(&bytes).map_err(|_| StateError::Deserialization)
}

/// Returns a tuple of the bytes and length of the argument.
/// `host_ptr` is encoded using Big Endian as an i64.
#[must_use]
pub fn into_bytes(host_ptr: HostPtr) -> Vec<u8> {
    // grab length from ptrArg
    let (ptr, len) = split_host_ptr(host_ptr);
    let value = unsafe { std::slice::from_raw_parts(ptr as *const u8, len) };
    value.to_vec()
}

/* memory functions ------------------------------------------- */
/// Allocate memory into the instance of Program and return the offset to the
/// start of the block.
/// # Panics
/// Panics if the pointer exceeds the maximum size of an i64.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> *mut u8 {
    // create a new mutable buffer with capacity `len`
    let mut buf = Vec::with_capacity(len);
    // take a mutable pointer to the buffer
    let ptr = buf.as_mut_ptr();
    // ensure memory pointer is fits in an i64
    // to avoid potential issues when passing
    // across wasm boundary
    assert!(i64::try_from(ptr as u64).is_ok());
    // take ownership of the memory block and
    // ensure that its destructor is not
    // called when the object goes out of scope
    // at the end of the function
    std::mem::forget(buf);
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
