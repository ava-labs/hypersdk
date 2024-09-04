// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{Address, HostPtr};
use cfg_if::cfg_if;

pub const BALANCE_PREFIX: u8 = 0;
pub const SEND_PREFIX: u8 = 1;
pub const CALL_FUNCTION_PREFIX: u8 = 2;
pub const DEPLOY_PREFIX: u8 = 3;

pub struct StateAccessor;

#[cfg(feature = "test")]
impl StateAccessor {
    pub fn put(_ptr: *const u8, _len: usize) {
        // happens on context drop() -> cache drop() -> flush()
        // this means this function wont do anything
        todo!()
    }

    pub fn get_bytes(_ptr: *const u8, _len: usize) -> HostPtr {
        // if calling get_bytes, not found in cache
        HostPtr::null()
    }
}

#[cfg(not(feature = "test"))]
impl StateAccessor {
    pub fn new() -> Self {
        StateAccessor
    }

    pub fn put(ptr: *const u8, len: usize) {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put"]
            fn put(ptr: *const u8, len: usize);
        }

        unsafe {
            put(ptr, len);
        }
    }

    pub fn get_bytes(ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { get_bytes(ptr, len) }
    }
}

#[cfg(feature = "test")]
use std::collections::HashMap;

#[cfg(feature = "test")]
#[derive(Clone, Debug)]
pub struct MockState {
    state: hashbrown::HashMap<Vec<u8>, Vec<u8>>,
}

#[cfg(feature = "test")]
impl Default for MockState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(feature = "test")]
impl MockState {
    pub fn new() -> Self {
        Self {
            state: hashbrown::HashMap::new(),
        }
    }

    pub fn get(&self, key: &[u8]) -> HostPtr {
        match self.state.get(key) {
            Some(val) => {
                let ptr = crate::memory::alloc(val.len());
                unsafe {
                    std::ptr::copy(val.as_ptr(), ptr.as_ptr().cast_mut(), val.len());
                }
                ptr
            }
            None => HostPtr::null(),
        }
    }

    pub fn put(&mut self, key: &[u8], value: Vec<u8>) {
        self.state.insert(key.into(), value);
    }

    pub fn remove(&mut self, key: &[u8]) {
        self.state.remove(key);
    }

    pub fn len(&self) -> usize {
        self.state.len()
    }
}

cfg_if! {
    if #[cfg(feature = "test")] {
        #[derive(Clone)]
        #[cfg_attr(feature = "debug", derive(Debug))]
        pub struct Accessor {
            pub state: MockState,
            // countes the number of deploys to generate a unique address
            pub deploys: u8,
        }
    } else {
        #[derive(Clone)]
        #[cfg_attr(feature = "debug", derive(Debug))]
        pub struct Accessor;
    }
}

#[cfg(feature = "test")]
impl Default for Accessor {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(feature = "test")]
impl Accessor {
    pub fn new() -> Self {
        Accessor {
            state: MockState::new(),
            // not sure why this breaks when 0?
            deploys: 1,
        }
    }

    pub fn new_deploy_address(&mut self) -> Address {
        let address: [u8; 33] = [self.deploys; 33];
        assert!(self.deploys != 255, "Too many deploys");
        self.deploys += 1;
        Address::new(address)
    }

    pub fn deploy(&mut self, ptr: *const u8, len: usize) -> HostPtr {
        let key = unsafe { std::slice::from_raw_parts(ptr, len) };
        let key = [DEPLOY_PREFIX]
            .iter()
            .chain(key.iter())
            .copied()
            .collect::<Vec<u8>>();
        let val = self.state.get(&key);

        assert!(
            !val.is_null(),
            "Deploy function not mocked. Please mock the function call."
        );

        val
    }

    pub fn call_program(&self, ptr: *const u8, len: usize) -> HostPtr {
        let key = unsafe { std::slice::from_raw_parts(ptr, len) };
        let key = [CALL_FUNCTION_PREFIX]
            .iter()
            .chain(key.iter())
            .copied()
            .collect::<Vec<u8>>();
        let val = self.state.get(&key);

        assert!(
            !val.is_null(),
            "Call function not mocked. Please mock the function call."
        );

        val
    }

    pub fn get_balance(&self, ptr: *const u8, len: usize) -> HostPtr {
        let key = unsafe { std::slice::from_raw_parts(ptr, len) };
        // balance prefix + key
        let key = [BALANCE_PREFIX]
            .iter()
            .chain(key.iter())
            .copied()
            .collect::<Vec<u8>>();

        let host_ptr = self.state.get(&key);
        assert!(
            !host_ptr.is_null(),
            "get_balance not mocked. Please mock the function call."
        );

        host_ptr
    }

    #[allow(clippy::unused_self)]
    pub fn get_remaining_fuel(&self) -> HostPtr {
        panic!("get_remaining_fuel not implemented in the test context");
    }

    pub fn send_value(&self, ptr: *const u8, len: usize) -> HostPtr {
        let key = unsafe { std::slice::from_raw_parts(ptr, len) };
        // send prefix + key
        let key = [SEND_PREFIX]
            .iter()
            .chain(key.iter())
            .copied()
            .collect::<Vec<u8>>();

        let host_ptr = self.state.get(&key);
        assert!(
            !host_ptr.is_null(),
            "send_value not mocked. Please mock the function call."
        );

        host_ptr
    }
}

#[cfg(not(feature = "test"))]
impl Accessor {
    pub fn new() -> Self {
        Accessor
    }

    #[allow(clippy::unused_self)]
    pub fn deploy(&self, ptr: *const u8, len: usize) -> HostPtr {
        use crate::HostPtr;
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "deploy"]
            fn deploy(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { deploy(ptr, len) }
    }

    #[allow(clippy::unused_self)]
    pub fn call_program(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "call_program"]
            fn call_program(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { call_program(ptr, len) }
    }

    #[allow(clippy::unused_self)]
    pub fn get_balance(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "balance")]
        extern "C" {
            #[link_name = "get"]
            fn get(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { get(ptr, len) }
    }

    #[allow(clippy::unused_self)]
    pub fn get_remaining_fuel(&self) -> HostPtr {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "remaining_fuel"]
            fn get_remaining_fuel() -> HostPtr;
        }

        unsafe { get_remaining_fuel() }
    }

    #[allow(clippy::unused_self)]
    pub fn send_value(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "balance")]
        extern "C" {
            #[link_name = "send"]
            fn send_value(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { send_value(ptr, len) }
    }
}
