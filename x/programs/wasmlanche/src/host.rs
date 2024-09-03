use core::result;

use borsh::BorshSerialize;

use crate::{host, memory::wasmlanche_alloc, state::MockState, Address, Gas, HostPtr};

#[cfg(not(feature = "unit_tests"))]
pub struct StateAccessor;

#[cfg(not(feature = "unit_tests"))]
impl StateAccessor {
    pub fn new() -> Self {
        StateAccessor
    }

    pub fn put(&self, ptr: *const u8, len: usize) {
        println!("Host function context");
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "put"]
            fn put(ptr: *const u8, len: usize);
        }

        unsafe {
            put(ptr, len);
        }
    }

    pub fn get_bytes(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { get_bytes(ptr, len) }
    }
}

#[cfg(feature = "unit_tests")]
pub struct StateAccessor {
    state: MockState,
}

#[cfg(feature = "unit_tests")]
impl StateAccessor {
    pub fn new() -> Self {
        StateAccessor {
            state: MockState::new(),
        }
    }

    pub fn put(&self, _ptr: *const u8, _len: usize) {
        println!("Test function context");
        crate::dbg!("putting data in the test(ps: its fake ::::))))");
        // todo: putting happens on flush, when cache is dropped which happens when context is dropped
        // this means this function wont do anything
    }

    pub fn get_bytes(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        println!("Test function context");
        crate::dbg!("getting data in the test(ps: its fake ::::))))");
        // todo: if its calling get_bytes, it's not in the cache.
        HostPtr::null()
    }
}

#[cfg(not(feature = "unit_tests"))]
#[derive(Clone)]
pub struct HostAccessor;

#[cfg(feature = "unit_tests")]
#[derive(Clone)]
pub struct HostAccessor {
    pub state: MockState,
}

const BALANCE_PREFIX: u8 = 0;
const PROGRAM_PREFIX: u8 = 1;
const SEND_PREFIX: u8 = 2;

#[cfg(feature = "unit_tests")]
impl HostAccessor {
    pub fn new() -> Self {
        HostAccessor {
            state: MockState::new(),
        }
    }

    pub fn deploy(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        // creates a host function pointing to an account
        println!("Test function context");
        crate::dbg!("deploying program in the test(ps: its fake ::::))))");
        let address = [1_u8; 33];
        let host_ptr = wasmlanche_alloc(address.len());
        unsafe {
            std::ptr::copy(
                address.as_ptr(),
                host_ptr.as_ptr() as *mut u8,
                address.len(),
            );
        };

        host_ptr
    }

    pub fn call_program(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        let slice = unsafe { std::slice::from_raw_parts(_ptr, _len) };
        println!("from raw parts: {:?}", slice);
        let val = self.state.get(slice);

        assert!(
            !val.is_null(),
            "Call function not mocked. Please mock the function call."
        );

        val
    }

    pub fn get_balance(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        let key = unsafe { std::slice::from_raw_parts(_ptr, _len) };
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

    pub fn get_remaining_fuel(&self) -> HostPtr {
        panic!("get_remaining_fuel not implemented in the test context");
    }

    pub fn send_value(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        let key = unsafe { std::slice::from_raw_parts(_ptr, _len) };
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

#[cfg(not(feature = "unit_tests"))]
impl HostAccessor {
    pub fn new() -> Self {
        HostAccessor
    }

    pub fn deploy(&self, ptr: *const u8, len: usize) -> HostPtr {
        use crate::HostPtr;
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "deploy"]
            fn deploy(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { deploy(ptr, len) }
    }

    pub fn call_program(&self, ptr: *const u8, len: usize) -> HostPtr {
        println!("Host function context");
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "call_program"]
            fn call_program(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { call_program(ptr, len) }
    }

    pub fn get_balance(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "balance")]
        extern "C" {
            #[link_name = "get"]
            fn get(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { get(ptr, len) }
    }

    pub fn get_remaining_fuel(&self) -> HostPtr {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "remaining_fuel"]
            fn get_remaining_fuel() -> HostPtr;
        }

        unsafe { get_remaining_fuel() }
    }

    pub fn send_value(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "balance")]
        extern "C" {
            #[link_name = "send"]
            fn send_value(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe { send_value(ptr, len) }
    }
}
