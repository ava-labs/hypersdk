use std::marker::PhantomData;

use crate::{dbg, host, memory::wasmlanche_alloc, state::MockState, HostPtr};


#[cfg(not(feature = "unit_tests"))]
pub struct FunctionContext;

#[cfg(feature = "unit_tests")]
pub struct FunctionContext {
    state: MockState,
}

#[cfg(feature = "unit_tests")]
impl FunctionContext{
    pub fn new() -> Self {
        FunctionContext {
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
        HostPtr(std::ptr::null())
    }

    pub fn deploy(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        // creates a host function pointing to an account
        println!("Test function context");
        crate::dbg!("deploying program in the test(ps: its fake ::::))))"); 
        let address = [1_u8; 33];
        let host_ptr = wasmlanche_alloc(address.len());
        unsafe {
            std::ptr::copy(address.as_ptr(), host_ptr.0 as *mut u8, address.len());
        };

        host_ptr
    }

    pub fn call_program(&self) {
        println!("Test function context");
    }

    pub fn get_balance(&self, _ptr: *const u8, _len: usize) -> HostPtr {
        println!("Test function context");
        crate::dbg!("getting balance in the test(ps: its fake ::::))))");

        // TODO: temp value for now
        let balance = 1000_u64;
        let host_ptr = wasmlanche_alloc(std::mem::size_of::<u64>());
        unsafe {
            std::ptr::copy(&balance as *const u64 as *const u8, host_ptr.0 as *mut u8, std::mem::size_of::<u64>());
        };

        host_ptr
    }

}


#[cfg(not(feature = "unit_tests"))]
impl FunctionContext {
    pub fn new() -> Self {
        FunctionContext
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
        crate::dbg!("getting data in the host");
        #[link(wasm_import_module = "state")]
        extern "C" {
            #[link_name = "get"]
            fn get_bytes(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe {
            get_bytes(ptr, len)
        }
    }

    pub fn deploy(&self, ptr: *const u8, len: usize) -> HostPtr {
        crate::dbg!("deploying program in the host");

        use crate::HostPtr;
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "deploy"]
            fn deploy(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe {
            deploy(ptr, len)
        }
    }

    pub fn call_program(&self) {
        println!("Host function context");
    }


    pub fn get_balance(&self, ptr: *const u8, len: usize) -> HostPtr {
        #[link(wasm_import_module = "balance")]
        extern "C" {
            #[link_name = "get"]
            fn get(ptr: *const u8, len: usize) -> HostPtr;
        }

        unsafe {
            get(ptr, len)
        }
    }
}