// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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

#[doc(hidden)]
/// A pointer where data points to the host.
#[cfg_attr(feature = "debug", derive(Debug))]
#[repr(transparent)]
pub struct HostPtr(pub *const u8);

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

        let len = remove(self.0);
        let layout = Layout::array::<u8>(len).expect("capacity overflow");

        unsafe { std::alloc::dealloc(self.0.cast_mut(), layout) };
    }
}

impl From<HostPtr> for Vec<u8> {
    fn from(host_ptr: HostPtr) -> Self {
        // drop will dealloc the bytes
        let host_ptr = ManuallyDrop::new(host_ptr);

        let len = remove(host_ptr.0);

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
extern "C" fn alloc(len: usize) -> HostPtr {
   wasmlanche_alloc(len)
}

// TODO: putting this function here so we get consistent behavior on FunctionContext
pub fn wasmlanche_alloc(len: usize) -> HostPtr {
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

fn remove(ptr: *const u8) -> usize {
    ALLOCATIONS
        .with_borrow_mut(|allocations| allocations.remove(&ptr))
        .expect("attempted to drop invalid host pointer")
}

#[cfg(test)]
mod tests {
    #![allow(clippy::useless_vec)]
    use super::*;

    #[test]
    #[should_panic(expected = "attempted to deref invalid host pointer")]
    fn deref_untracked_pointer() {
        let ptr = HostPtr(std::ptr::null());
        let _ = &*ptr;
    }

    #[test]
    #[should_panic(expected = "attempted to drop invalid host pointer")]
    fn drop_untracked_pointer() {
        let data = vec![0xff, 0xaa];
        let ptr = HostPtr(data.as_ptr());
        drop(ptr);
    }

    #[test]
    fn deref_tracked_pointer() {
        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        ALLOCATIONS.with_borrow_mut(move |allocations| {
            allocations.insert(ptr, data.len());
        });

        assert_eq!(&*HostPtr(ptr), &cloned);
    }

    #[test]
    fn deref_is_borrow() {
        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        ALLOCATIONS.with_borrow_mut(move |allocations| {
            allocations.insert(ptr, data.len());
        });

        let host_ptr = ManuallyDrop::new(HostPtr(ptr));
        assert_eq!(&**host_ptr, &cloned);
        let host_ptr = HostPtr(ptr);
        assert_eq!(&*host_ptr, &cloned);
    }

    #[test]
    fn host_pointer_to_vec_takes_bytes() {
        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        ALLOCATIONS.with_borrow_mut(move |allocations| {
            allocations.insert(ptr, data.len());
        });

        assert_eq!(Vec::from(HostPtr(ptr)), cloned);

        ALLOCATIONS.with_borrow(|allocations| {
            assert!(allocations.get(&ptr).is_none());
        });
    }

    #[test]
    fn dropping_host_pointer_deallocates() {
        let data = vec![0xff];
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        ALLOCATIONS.with_borrow_mut(move |allocations| {
            allocations.insert(ptr, data.len());
        });

        drop(HostPtr(ptr));

        ALLOCATIONS.with_borrow(|allocations| {
            assert!(allocations.get(&ptr).is_none());
        });

        // overwrites old allocation
        let data = vec![0x00];
        assert_eq!(data.as_ptr(), ptr);
    }

    #[test]
    #[should_panic = "cannot allocate 0 sized data"]
    fn zero_allocation_panics() {
        alloc(0);
    }

    #[test]
    fn allocate_normal_length_data() {
        let len = 1024;
        let data: Vec<_> = (u8::MIN..=u8::MAX).cycle().take(len).collect();
        let ptr = alloc(len);
        unsafe { std::ptr::copy(data.as_ptr(), ptr.0.cast_mut(), data.len()) }

        assert_eq!(&*ptr, &*data);
    }
}
