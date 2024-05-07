//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

use crate::state::Error as StateError;
use borsh::{from_slice, BorshDeserialize};
use std::{alloc::Layout, cell::RefCell, collections::HashMap};

thread_local! {
    /// Map of pointer to the length of its content on the heap
    static GLOBAL_STORE: RefCell<HashMap<*const u8, usize>> = RefCell::new(HashMap::new());
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
pub fn from_host_ptr<V>(ptr: *const u8) -> Result<V, StateError>
where
    V: BorshDeserialize,
{
    match into_bytes(ptr) {
        Some(bytes) => from_slice::<V>(&bytes).map_err(|_| StateError::Deserialization),
        None => Err(StateError::InvalidPointer),
    }
}

/// Reconstructs the vec from the pointer with the length given by the store
/// `host_ptr` is encoded using Big Endian as an i64.
#[must_use]
pub fn into_bytes(ptr: *const u8) -> Option<Vec<u8>> {
    GLOBAL_STORE
        .with_borrow_mut(|s| s.remove(&ptr))
        .map(|len| unsafe { std::vec::Vec::from_raw_parts(ptr.cast_mut(), len, len) })
}

/* memory functions ------------------------------------------- */
/// Allocate memory into the instance of Program and return the offset to the
/// start of the block.
/// # Panics
/// Panics if the pointer exceeds the maximum size of an isize or that the allocated memory is null.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> *mut u8 {
    assert!(len > 0, "cannot allocate 0 sized data");
    // can only fail if `len > isize::MAX` for u8
    let layout = Layout::array::<u8>(len).expect("capacity overflow");
    // take a mutable pointer to the layout
    let ptr = unsafe { std::alloc::alloc(layout) };
    if ptr.is_null() {
        std::alloc::handle_alloc_error(layout);
    }
    // keep track of the pointer and the length of the allocated data
    GLOBAL_STORE.with_borrow_mut(|s| s.insert(ptr, len));
    // return the pointer so the runtime
    // can write data at this offset
    ptr
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
        unsafe { std::ptr::copy(vec.as_ptr(), ptr, vec.len()) }
        let val = into_bytes(ptr).unwrap();
        assert_eq!(val, vec);
        assert!(GLOBAL_STORE.with_borrow(|s| s.get(&(ptr.cast_const())).is_none()));
    }

    #[test]
    #[should_panic = "cannot allocate 0 sized data"]
    fn zero_allocation_panics() {
        alloc(0);
    }

    #[test]
    #[should_panic = "capacity overflow"]
    fn big_allocation_fails() {
        // see https://doc.rust-lang.org/1.77.2/std/alloc/struct.Layout.html#method.array
        alloc(isize::MAX as usize + 1);
    }

    // TODO these two tests make the code abort and not panic, it's hard to write an assertion here
    // #[test]
    // #[should_panic]
    // fn two_big_allocation_fails() {
    //     // see https://doc.rust-lang.org/1.77.2/std/alloc/struct.Layout.html#method.array
    //     alloc((isize::MAX / 2) as usize + 1);
    //     alloc((isize::MAX / 2) as usize + 1);
    // }

    //     #[test]
    //     #[should_panic]
    //     fn null_pointer_allocation() {
    //         // see https://doc.rust-lang.org/1.77.2/std/alloc/struct.Layout.html#method.array
    //         alloc(isize::MAX as usize);
    //     }
}
