// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#![cfg(not(target_arch = "wasm32"))]

use std::ops::{Deref, DerefMut};

const TEST_PKG: &str = "test-crate";

#[test]
fn public_functions() {
    let mut test_crate = build_test_crate();

    let context_ptr = test_crate.allocate_context();
    assert!(test_crate.always_true(context_ptr));

    let context_ptr = test_crate.allocate_context();
    let combined_binary_digits = test_crate.combine_last_bit_of_each_id_byte(context_ptr);
    assert_eq!(combined_binary_digits, u32::MAX);
}

#[test]
// the failure message is from the `expect` in this file
#[should_panic = "failed to allocate memory"]
fn allocate_zero() {
    let mut test_crate = build_test_crate();
    test_crate.allocate(Vec::new());
}

const ALLOCATION_MAP_OVERHEAD: usize = 48;

#[test]
fn allocate_data_size() {
    let mut test_crate = build_test_crate();
    let context_ptr = test_crate.allocate_context();
    let highest_address = test_crate.highest_allocated_address(context_ptr);
    let memory = test_crate.memory();
    let room = memory.data_size(test_crate.store_mut()) - highest_address - ALLOCATION_MAP_OVERHEAD;
    let data = vec![0xaa; room];
    let ptr = test_crate.allocate(data.clone()) as usize;
    let memory = memory.data(test_crate.store_mut());
    assert_eq!(&memory[ptr..ptr + room], &data);
}

#[test]
#[should_panic]
fn allocate_data_size_plus_one() {
    let mut test_crate = build_test_crate();
    let context_ptr = test_crate.allocate_context();
    let highest_address = test_crate.highest_allocated_address(context_ptr);
    let memory = test_crate.memory();
    let room = memory.data_size(test_crate.store_mut()) - highest_address - ALLOCATION_MAP_OVERHEAD;
    let data = vec![0xaa; room + 1];
    test_crate.allocate(data.clone());
}

fn build_test_crate() -> TestCrate {
    let mut inner = wasmlanche_test::Builder::new(TEST_PKG).build();

    let highest_address_func = inner.get_user_defined_typed_func("highest_allocated_address");

    let always_true_func = inner.get_user_defined_typed_func("always_true");
    let combine_last_bit_of_each_id_byte_func =
        inner.get_user_defined_typed_func("combine_last_bit_of_each_id_byte");

    TestCrate {
        inner,
        highest_address_func,
        always_true_func,
        combine_last_bit_of_each_id_byte_func,
    }
}

struct TestCrate {
    inner: wasmlanche_test::TestCrate,
    highest_address_func: wasmlanche_test::UserDefinedFn,
    always_true_func: wasmlanche_test::UserDefinedFn,
    combine_last_bit_of_each_id_byte_func: wasmlanche_test::UserDefinedFn,
}

impl Deref for TestCrate {
    type Target = wasmlanche_test::TestCrate;

    fn deref(&self) -> &Self::Target {
        &self.inner
    }
}

impl DerefMut for TestCrate {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.inner
    }
}

impl TestCrate {
    fn highest_allocated_address(&mut self, ptr: wasmlanche_test::UserDefinedFnParam) -> usize {
        let Self {
            highest_address_func,
            inner,
            ..
        } = self;

        highest_address_func
            .call(inner.store_mut(), ptr)
            .expect("failed to call `highest_allocated_address` function");

        let result = inner
            .store_mut()
            .data_mut()
            .take_result()
            .expect("highest_allocated_address should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }

    fn always_true(&mut self, ptr: wasmlanche_test::UserDefinedFnParam) -> bool {
        let Self {
            always_true_func,
            inner,
            ..
        } = self;

        always_true_func
            .call(inner.store_mut(), ptr)
            .expect("failed to call `always_true` function");
        let result = inner
            .store_mut()
            .data_mut()
            .take_result()
            .expect("always_true should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }

    fn combine_last_bit_of_each_id_byte(
        &mut self,
        ptr: wasmlanche_test::UserDefinedFnParam,
    ) -> u32 {
        let Self {
            combine_last_bit_of_each_id_byte_func,
            inner,
            ..
        } = self;

        combine_last_bit_of_each_id_byte_func
            .call(inner.store_mut(), ptr)
            .expect("failed to call `combine_last_bit_of_each_id_byte` function");
        let result = inner
            .store_mut()
            .data_mut()
            .take_result()
            .expect("combine_last_bit_of_each_id_byte should always return something");
        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}
