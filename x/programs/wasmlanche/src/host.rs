// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

pub struct StateAccessor;

#[cfg(feature = "test")]
mod test_wrappers {
    use crate::host::StateAccessor;
    use crate::{Address, Gas, HostPtr};
    use core::cell::{Cell, RefCell};

    pub const BALANCE_PREFIX: u8 = 0;
    pub const SEND_PREFIX: u8 = 1;
    pub const CALL_FUNCTION_PREFIX: u8 = 2;
    pub const DEPLOY_PREFIX: u8 = 3;

    impl StateAccessor {
        pub fn put(_args: &[u8]) {
            // happens on context drop() -> cache drop() -> flush()
            // this means this function wont do anything
        }

        pub fn get_bytes(_args: &[u8]) -> HostPtr {
            // if calling get_bytes, not found in cache
            HostPtr::null()
        }
    }

    #[derive(Clone)]
    #[cfg_attr(feature = "debug", derive(Debug))]
    pub struct Accessor {
        pub state: MockState,
    }

    impl Default for Accessor {
        fn default() -> Self {
            Self::new()
        }
    }

    impl Accessor {
        pub fn new() -> Self {
            Accessor {
                state: MockState::new(),
            }
        }

        pub fn state(&self) -> &MockState {
            &self.state
        }

        pub fn new_deploy_address(&self) -> Address {
            let address: [u8; 33] = [self.state().deploy(); 33];
            Address::new(address)
        }

        pub fn deploy(&self, key: &[u8]) -> HostPtr {
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

        pub fn call_program(&self, args: &[u8]) -> HostPtr {
            let key = [CALL_FUNCTION_PREFIX]
                .iter()
                .chain(args.iter())
                .copied()
                .collect::<Vec<u8>>();
            let val = self.state.get(&key);

            assert!(
                !val.is_null(),
                "Call function not mocked. Please mock the function call."
            );

            val
        }

        pub fn get_balance(&self, args: &[u8]) -> HostPtr {
            // balance prefix + key
            let key = [BALANCE_PREFIX]
                .iter()
                .chain(args.iter())
                .copied()
                .collect::<Vec<u8>>();

            let host_ptr = self.state.get(&key);
            assert!(
                !host_ptr.is_null(),
                "get_balance not mocked. Please mock the function call."
            );

            host_ptr
        }

        pub fn get_remaining_fuel(&self) -> HostPtr {
            self.state().get_fuel()
        }

        pub fn send_value(&self, args: &[u8]) -> HostPtr {
            // send prefix + key
            let key = [SEND_PREFIX]
                .iter()
                .chain(args.iter())
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

    impl Default for MockState {
        fn default() -> Self {
            Self::new()
        }
    }

    #[derive(Clone, Debug)]
    pub struct MockState {
        state: RefCell<hashbrown::HashMap<Vec<u8>, Vec<u8>>>,
        deploys: Cell<u8>,
        fuel: Gas,
    }

    impl MockState {
        pub fn new() -> Self {
            Self {
                state: RefCell::new(hashbrown::HashMap::new()),
                deploys: Cell::new(0),
                fuel: u64::MAX,
            }
        }

        pub fn get(&self, key: &[u8]) -> HostPtr {
            match self.state.borrow().get(key) {
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

        pub fn put(&self, key: &[u8], value: Vec<u8>) {
            self.state.borrow_mut().insert(key.into(), value);
        }

        pub fn deploy(&self) -> u8 {
            self.deploys.set(self.deploys.get() + 1);
            self.deploys.get()
        }

        pub fn get_fuel(&self) -> HostPtr {
            let fuel_bytes = borsh::to_vec(&self.fuel).expect("failed to serialize");
            let ptr = crate::memory::alloc(fuel_bytes.len());
            unsafe {
                std::ptr::copy(
                    fuel_bytes.as_ptr(),
                    ptr.as_ptr().cast_mut(),
                    fuel_bytes.len(),
                );
            }
            ptr
        }
    }
}

#[cfg(not(feature = "test"))]
mod external_wrappers {
    use crate::host::StateAccessor;
    use crate::HostPtr;

    impl StateAccessor {
        pub fn put(args: &[u8]) {
            #[link(wasm_import_module = "state")]
            extern "C" {
                #[link_name = "put"]
                fn put(ptr: *const u8, len: usize);
            }

            unsafe {
                put(args.as_ptr(), args.len());
            }
        }

        pub fn get_bytes(args: &[u8]) -> HostPtr {
            #[link(wasm_import_module = "state")]
            extern "C" {
                #[link_name = "get"]
                fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
            }

            unsafe { get_bytes(args.as_ptr(), args.len()) }
        }
    }

    #[derive(Clone)]
    #[cfg_attr(feature = "debug", derive(Debug))]
    pub struct Accessor;

    impl Accessor {
        #![allow(clippy::unused_self)]

        pub fn new() -> Self {
            Accessor
        }

        pub fn deploy(&self, args: &[u8]) -> HostPtr {
            use crate::HostPtr;
            #[link(wasm_import_module = "program")]
            extern "C" {
                #[link_name = "deploy"]
                fn deploy(ptr: *const u8, len: usize) -> HostPtr;
            }

            unsafe { deploy(args.as_ptr(), args.len()) }
        }

        pub fn call_program(&self, args: &[u8]) -> HostPtr {
            #[link(wasm_import_module = "program")]
            extern "C" {
                #[link_name = "call_program"]
                fn call_program(ptr: *const u8, len: usize) -> HostPtr;
            }

            unsafe { call_program(args.as_ptr(), args.len()) }
        }

        pub fn get_balance(&self, args: &[u8]) -> HostPtr {
            #[link(wasm_import_module = "balance")]
            extern "C" {
                #[link_name = "get"]
                fn get(ptr: *const u8, len: usize) -> HostPtr;
            }

            unsafe { get(args.as_ptr(), args.len()) }
        }

        pub fn get_remaining_fuel(&self) -> HostPtr {
            #[link(wasm_import_module = "program")]
            extern "C" {
                #[link_name = "remaining_fuel"]
                fn get_remaining_fuel() -> HostPtr;
            }

            unsafe { get_remaining_fuel() }
        }

        pub fn send_value(&self, args: &[u8]) -> HostPtr {
            #[link(wasm_import_module = "balance")]
            extern "C" {
                #[link_name = "send"]
                fn send_value(ptr: *const u8, len: usize) -> HostPtr;
            }

            unsafe { send_value(args.as_ptr(), args.len()) }
        }
    }
}

#[cfg(feature = "test")]
pub use test_wrappers::*;

#[cfg(not(feature = "test"))]
pub use external_wrappers::*;
