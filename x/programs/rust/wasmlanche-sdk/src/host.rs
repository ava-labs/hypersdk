use std::marker::PhantomData;

use crate::{dbg, host, memory::wasmlanche_alloc, HostPtr};


pub struct HostFunctionContext;
pub struct TestFunctionContext;

#[cfg(not(feature = "unit_tests"))]
pub struct FunctionContext<Functions = HostFunctionContext> {
    functions: PhantomData<Functions>,
}

#[cfg(feature = "unit_tests")]
pub struct FunctionContext<Functions = TestFunctionContext> {
    functions: PhantomData<Functions>,
}

impl Default for FunctionContext {
    fn default() -> Self {
        Self {
            functions: PhantomData,
        }
    }
}

impl FunctionContext<TestFunctionContext> {
    pub fn put(&self) {
        println!("Test function context");
    }
    
    pub fn get(&self) {
        println!("Test function context");
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
}


impl FunctionContext<HostFunctionContext> {
    pub fn put(&self) {
        println!("Host function context");
    }
    
    pub fn get(&self) {
        println!("Host function context");
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
}