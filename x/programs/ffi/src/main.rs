use std::ffi::CString;

use libc::{c_char, c_int, c_uchar, c_uint, c_void};

mod types;
mod state;
use types::{ExecutionRequest, Response, SimpleMutable, SimulatorContext, ID, Address};


#[link(name = "simulator", kind = "dylib")]
extern "C" {
    fn Execute(db: *const SimpleMutable, ctx: *const SimulatorContext, request: *const ExecutionRequest) -> Response;
}

type RustFunction = unsafe extern "C" fn(c_int, c_int, *mut c_void) -> c_int;

unsafe extern "C" fn trampoline(a: c_int, b: c_int, data: *mut c_void) -> c_int {
    // Convert the raw pointer back to a Box and call the closure
    let closure: &mut Box<dyn FnMut(c_int, c_int) -> c_int> = &mut *(data as *mut Box<dyn FnMut(c_int, c_int) -> c_int>);
    closure(a, b)
}

fn main() {
    let method = CString::new("myMethod").expect("CString::new failed");
    let params: Vec<u8> = vec![1, 2, 3, 4, 5]; // Example byte array
    let param_length = params.len() as c_uint;
    let max_gas = 1000;

    let mut multiplier = 2;
    let closure = move |a: c_int, b: c_int| -> c_int {
        println!("Rust closure called with {} and {}", a, b);
        (a * b) * multiplier
    };

    let execution_params = ExecutionRequest {
        method: method.as_ptr(),
        params: params.as_ptr(),
        param_length,
        max_gas,
    };

    let ctx = SimulatorContext {
        program_address: Address { address: [0; 33] },
        actor_address: Address { address: [0; 33] },
        height: 0,
        timestamp: 0,
    };
      // Box the closure to make it a trait object
      let boxed_closure: Box<dyn FnMut(c_int, c_int) -> c_int> = Box::new(closure);
      let closure_ptr: *mut c_void = Box::into_raw(Box::new(boxed_closure)) as *mut c_void;
  
    println!("Calling the Execute function");
    let response = unsafe {
        Execute(
            &SimpleMutable { value: 0 },
            &ctx, &execution_params)
    };

    println!("The response id is: {}", response.id);
}
