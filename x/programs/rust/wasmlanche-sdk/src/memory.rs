//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

use std::{alloc::Layout, cell::RefCell, collections::HashMap, mem::ManuallyDrop, ops::Deref};

thread_local! {
    /// Map of pointer to the length of its content on the heap
    static ALLOCATIONS: RefCell<HashMap<*const u8, usize>> = RefCell::new(HashMap::new());
}

#[cfg_attr(feature = "debug", derive(Debug))]
#[repr(transparent)]
pub struct HostPtr(*const u8);

impl Deref for HostPtr {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        let len = ALLOCATIONS
            .with_borrow(|allocations| allocations.get(&self.0).copied())
            .expect("attempted to deref invalid host pointer");

        unsafe { std::slice::from_raw_parts(self.0, len) }
    }
}

impl Drop for HostPtr {
    fn drop(&mut self) {
        if self.is_null() {
            return;
        }

        let len = ALLOCATIONS
            .with_borrow_mut(|allocations| allocations.remove(&self.0))
            .expect("attempted to drop invalid host pointer");
        let layout = Layout::array::<u8>(len).expect("capacity overflow");

        unsafe { std::alloc::dealloc(self.0.cast_mut(), layout) };
    }
}

impl From<HostPtr> for Vec<u8> {
    fn from(host_ptr: HostPtr) -> Self {
        // drop will dealloc the bytes
        let host_ptr = ManuallyDrop::new(host_ptr);

        let len = ALLOCATIONS
            .with_borrow_mut(|allocations| allocations.remove(&host_ptr.0))
            .expect("attempted to drop invalid host pointer");

        unsafe { Vec::from_raw_parts(host_ptr.0.cast_mut(), len, len) }
    }
}

impl HostPtr {
    #[must_use]
    pub fn is_null(&self) -> bool {
        self.0.is_null()
    }
}

/// Allocate memory into the instance of Program and return the offset to the
/// start of the block.
/// # Panics
/// Panics if the pointer exceeds the maximum size of an isize or that the allocated memory is null.
#[no_mangle]
pub extern "C" fn alloc(len: usize) -> HostPtr {
    assert!(len > 0, "cannot allocate 0 sized data");
    // can only fail if `len > isize::MAX` for u8
    let layout = Layout::array::<u8>(len).expect("capacity overflow");

    let ptr = unsafe { std::alloc::alloc(layout) };

    if ptr.is_null() {
        std::alloc::handle_alloc_error(layout);
    }

    ALLOCATIONS.with_borrow_mut(|s| s.insert(ptr, len));

    HostPtr(ptr.cast_const())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn data_allocation() {
        let len = 1024;
        let ptr = alloc(len);
        let vec: Vec<_> = (u8::MIN..=u8::MAX).cycle().take(len).collect();
        unsafe { std::ptr::copy(vec.as_ptr(), ptr.0.cast_mut(), vec.len()) }
        assert_eq!(*ptr, vec);

        let inner = ptr.0;
        drop(ptr);

        ALLOCATIONS.with_borrow(|map| assert!(!map.contains_key(&inner)));
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
