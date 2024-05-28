//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

use std::{alloc::Layout, cell::RefCell, collections::HashMap};

thread_local! {
    /// Map of pointer to the length of its content on the heap
    static GLOBAL_STORE: RefCell<HashMap<*const u8, usize>> = RefCell::new(HashMap::new());
}

/// Reconstructs the vec that was created using the [alloc] function.
/// # Panics
/// If you are trying to dereference data that has already been dereferenced
/// or the pointer is invalid (wasn't created with [alloc])
#[must_use]
pub fn deref_bytes(ptr: *const u8) -> Vec<u8> {
    GLOBAL_STORE
        .with_borrow_mut(|s| s.remove(&ptr))
        // SAFETY:
        // the space is reserved by the `alloc` function
        .map(|len| unsafe { std::vec::Vec::from_raw_parts(ptr.cast_mut(), len, len) })
        .expect("value missing or taken")
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

    let ptr = unsafe { std::alloc::alloc(layout) };

    if ptr.is_null() {
        std::alloc::handle_alloc_error(layout);
    }

    GLOBAL_STORE.with_borrow_mut(|s| s.insert(ptr, len));

    ptr
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn data_allocation() {
        let len = 1024;
        let ptr = alloc(len);
        let vec: Vec<_> = (u8::MIN..=u8::MAX).cycle().take(len).collect();
        unsafe { std::ptr::copy(vec.as_ptr(), ptr, vec.len()) }
        let val = deref_bytes(ptr);
        assert_eq!(val, vec);

        GLOBAL_STORE.with_borrow(|map| assert!(!map.contains_key(&ptr.cast_const())));
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
