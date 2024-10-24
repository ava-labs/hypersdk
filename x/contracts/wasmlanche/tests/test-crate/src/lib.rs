// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::{
    alloc::{GlobalAlloc, Layout, System},
    cell::UnsafeCell,
};
use wasmlanche::{public, Context};

struct HighestAllocatedAddress {
    value: UnsafeCell<usize>,
}

unsafe impl Sync for HighestAllocatedAddress {}

static HIGHEST_ALLOCATED_ADDRESS: HighestAllocatedAddress = HighestAllocatedAddress {
    value: UnsafeCell::new(0),
};

struct TrackingAllocator;

unsafe impl GlobalAlloc for TrackingAllocator {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        let ptr = System.alloc(layout);

        if ptr.is_null() {
            return ptr;
        }

        let addr = ptr as usize;
        let highest = HIGHEST_ALLOCATED_ADDRESS.value.get();

        if addr + layout.size() > *highest {
            *highest = addr + layout.size();
        }

        ptr
    }

    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        System.dealloc(ptr, layout);
    }
}

#[global_allocator]
static GLOBAL: TrackingAllocator = TrackingAllocator;

#[public]
pub fn highest_allocated_address(_: &mut Context) -> usize {
    unsafe { *HIGHEST_ALLOCATED_ADDRESS.value.get() }
}

#[public]
pub fn always_true(_: &mut Context) -> bool {
    true
}

#[public]
pub fn combine_last_bit_of_each_id_byte(context: &mut Context) -> u32 {
    context
        .contract_address()
        .into_iter()
        .map(|byte| byte as u32)
        .fold(0, |acc, byte| (acc << 1) + (byte & 1))
}

#[cfg(test)]
mod tests {
    use wasmlanche::{Address, Context};

    #[test]
    fn test_balance() {
        let address = Address::default();
        let mut context = Context::with_actor(address);
        let amount: u64 = 100;

        // set the balance
        context.mock_set_balance(address, amount);

        let balance = context.get_balance(address);
        assert_eq!(balance, amount);
    }
}
