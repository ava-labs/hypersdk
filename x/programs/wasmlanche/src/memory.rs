// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//! Temporary storage allocated during the Contract runtime.
//! The general pattern for handling memory is to have the
//! host allocate a block of memory and return a pointer to
//! the contract. These methods are unsafe as should be used
//! with caution.

extern crate alloc;

use alloc::{
    alloc::{alloc as allocate, dealloc as deallocate, handle_alloc_error, Layout},
    vec::Vec,
};
use core::{mem::ManuallyDrop, ops::Deref, slice};

mod allocations;

#[doc(hidden)]
/// A pointer where data points to the host.
#[cfg_attr(feature = "debug", derive(Debug))]
#[repr(transparent)]
pub struct HostPtr(*const u8);

impl Deref for HostPtr {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        let len = allocations::get(self.0).expect("attempted to deref invalid host pointer");
        unsafe { slice::from_raw_parts(self.0, len) }
    }
}

impl Drop for HostPtr {
    fn drop(&mut self) {
        if self.is_null() {
            return;
        }

        let len = allocations::remove(self.0).expect("attempted to drop invalid host pointer");
        let layout = Layout::array::<u8>(len).expect("capacity overflow");

        unsafe { deallocate(self.0.cast_mut(), layout) };
    }
}

impl From<HostPtr> for Vec<u8> {
    fn from(host_ptr: HostPtr) -> Self {
        // drop will dealloc the bytes
        let host_ptr = ManuallyDrop::new(host_ptr);

        let len = allocations::remove(host_ptr.0)
            .expect("attempted to convert invalid host pointer to a Vec");

        unsafe { Vec::from_raw_parts(host_ptr.0.cast_mut(), len, len) }
    }
}

impl HostPtr {
    #[must_use]
    pub fn is_null(&self) -> bool {
        self.0.is_null()
    }
}

#[cfg(feature = "test")]
impl HostPtr {
    #[must_use]
    pub fn null() -> Self {
        Self(core::ptr::null())
    }

    #[must_use]
    pub fn as_ptr(&self) -> *const u8 {
        self.0
    }
}

/// Allocate memory into the instance of Contract and return the offset to the
/// start of the block.
/// # Panics
/// Panics if the pointer exceeds the maximum size of an isize or that the allocated memory is null.
#[no_mangle]
pub(crate) extern "C-unwind" fn alloc(len: usize) -> HostPtr {
    assert!(len > 0, "cannot allocate 0 sized data");
    // can only fail if `len > isize::MAX` for u8
    let layout = Layout::array::<u8>(len).expect("capacity overflow");

    let ptr = unsafe { allocate(layout) };

    if ptr.is_null() {
        handle_alloc_error(layout);
    }

    allocations::insert(ptr, len);

    HostPtr(ptr.cast_const())
}

#[cfg(test)]
#[allow(clippy::useless_vec)]
mod tests {
    use super::*;
    use std::ptr;

    #[test]
    #[should_panic(expected = "attempted to deref invalid host pointer")]
    fn deref_untracked_pointer() {
        let ptr = HostPtr(ptr::null());
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

        allocations::insert(ptr, data.len());

        assert_eq!(&*HostPtr(ptr), &cloned);
    }

    #[test]
    fn deref_is_borrow() {
        let data = vec![0xff];
        let cloned = data.clone();
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        allocations::insert(ptr, data.len());

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

        allocations::insert(ptr, data.len());

        assert_eq!(Vec::from(HostPtr(ptr)), cloned);

        assert!(allocations::get(ptr).is_none());
    }

    #[test]
    #[should_panic(expected = "attempted to convert invalid host pointer to a Vec")]
    fn host_pointer_to_vec_panics_on_invalid_pointer() {
        let ptr = HostPtr(ptr::null());
        let _ = Vec::from(ptr);
    }

    #[test]
    fn dropping_host_pointer_deallocates() {
        let data = vec![0xff];
        let data = ManuallyDrop::new(data);
        let ptr = data.as_ptr();

        allocations::insert(ptr, data.len());

        drop(HostPtr(ptr));

        assert!(allocations::get(ptr).is_none());

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

        unsafe { ptr::copy(data.as_ptr(), ptr.0.cast_mut(), data.len()) }

        assert_eq!(&*ptr, &*data);
    }
}
