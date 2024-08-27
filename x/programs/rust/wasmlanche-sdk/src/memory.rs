// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//! Temporary storage allocated during the Program runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the program. These methods are unsafe as should be used
//! with caution.

extern crate alloc;

use alloc::{
    alloc::{alloc as allocate, dealloc as deallocate, handle_alloc_error, Layout},
    vec::Vec,
};
use core::{
    cell::{LazyCell, RefCell},
    mem::ManuallyDrop,
    ops::Deref,
    slice,
};
use hashbrown::HashMap;

/// Map of pointer to the length of its content on the heap

// TODO:
// `RefCell` shouldn't be necessary here since we need unsafe access anyway.
// However, it does guarantee no UB on a single-thread.

static mut ALLOCATIONS: LazyCell<RefCell<HashMap<*const u8, usize>>> =
    LazyCell::new(|| RefCell::new(HashMap::new()));

#[doc(hidden)]
/// A pointer where data points to the host.
#[cfg_attr(feature = "debug", derive(Debug))]
#[repr(transparent)]
pub struct HostPtr(*const u8);

impl Deref for HostPtr {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        unsafe {
            let len = ALLOCATIONS
                .borrow()
                .get(&self.0)
                .copied()
                .expect("attempted to deref invalid host pointer");

            slice::from_raw_parts(self.0, len)
        }
    }
}

impl Drop for HostPtr {
    fn drop(&mut self) {
        if self.is_null() {
            return;
        }

        let len = remove(self.0);
        let layout = Layout::array::<u8>(len).expect("capacity overflow");

        unsafe { deallocate(self.0.cast_mut(), layout) };
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
    assert!(len > 0, "cannot allocate 0 sized data");
    // can only fail if `len > isize::MAX` for u8
    let layout = Layout::array::<u8>(len).expect("capacity overflow");

    let ptr = unsafe { allocate(layout) };

    if ptr.is_null() {
        handle_alloc_error(layout);
    }

    unsafe {
        ALLOCATIONS.borrow_mut().insert(ptr, len);
    }

    HostPtr(ptr.cast_const())
}

fn remove(ptr: *const u8) -> usize {
    unsafe {
        ALLOCATIONS
            .borrow_mut()
            .remove(&ptr)
            .expect("attempted to drop invalid host pointer")
    }
}

#[cfg(test)]
#[allow(clippy::useless_vec)]
mod tests {
    use super::*;
    use std::ptr;

    // memory code is not safe to call from multiple threads
    static LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());

    #[test]
    #[should_panic(expected = "attempted to deref invalid host pointer")]
    fn deref_untracked_pointer() {
        let _lock = LOCK.lock();

        let ptr = HostPtr(ptr::null());
        let _ = &*ptr;
    }

    #[test]
    #[should_panic(expected = "attempted to drop invalid host pointer")]
    fn drop_untracked_pointer() {
        let _lock = LOCK.lock();

        let data = vec![0xff, 0xaa];
        let ptr = HostPtr(data.as_ptr());
        drop(ptr);
    }

    #[test]
    fn deref_tracked_pointer() {
        let _lock = LOCK.lock();

        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        unsafe {
            ALLOCATIONS.borrow_mut().insert(ptr, data.len());
        }

        assert_eq!(&*HostPtr(ptr), &cloned);
    }

    #[test]
    fn deref_is_borrow() {
        let _lock = LOCK.lock();

        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        unsafe {
            ALLOCATIONS.borrow_mut().insert(ptr, data.len());
        }

        let host_ptr = ManuallyDrop::new(HostPtr(ptr));
        assert_eq!(&**host_ptr, &cloned);
        let host_ptr = HostPtr(ptr);
        assert_eq!(&*host_ptr, &cloned);
    }

    #[test]
    fn host_pointer_to_vec_takes_bytes() {
        let _lock = LOCK.lock();

        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        unsafe {
            ALLOCATIONS.borrow_mut().insert(ptr, data.len());
        }

        assert_eq!(Vec::from(HostPtr(ptr)), cloned);

        unsafe {
            assert!(ALLOCATIONS.borrow().get(&ptr).is_none());
        }
    }

    #[test]
    fn dropping_host_pointer_deallocates() {
        let _lock = LOCK.lock();

        let data = vec![0xff];
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        unsafe {
            ALLOCATIONS.borrow_mut().insert(ptr, data.len());
        }

        drop(HostPtr(ptr));

        unsafe {
            assert!(ALLOCATIONS.borrow().get(&ptr).is_none());
        }

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
        let _lock = LOCK.lock();

        let len = 1024;
        let data: Vec<_> = (u8::MIN..=u8::MAX).cycle().take(len).collect();
        let ptr = alloc(len);
        unsafe { ptr::copy(data.as_ptr(), ptr.0.cast_mut(), data.len()) }

        assert_eq!(&*ptr, &*data);
    }
}
