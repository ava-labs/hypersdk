use std::ffi::CString;

use libc::{c_char, c_int, c_uchar, c_uint, c_void};

mod types;
mod state;
use types::{ExecutionRequest, Response, SimpleMutable, SimulatorContext, ID, Address};

// #[link(name = "simulator", kind = "dylib")]
// extern "C" {
//     fn Execute(db: *const SimpleMutable, ctx: *const SimulatorContext, request: *const ExecutionRequest) -> Response;
// }

// fn main() {
//     let method = CString::new("myMethod").expect("CString::new failed");
//     let params: Vec<u8> = vec![1, 2, 3, 4, 5]; // Example byte array
//     let param_length = params.len() as c_uint;
//     let max_gas = 1000;

//     let execution_params = ExecutionRequest {
//         method: method.as_ptr(),
//         params: params.as_ptr(),
//         param_length,
//         max_gas,
//     };

//     let ctx = SimulatorContext {
//         program_address: Address { address: [0; 33] },
//         actor_address: Address { address: [0; 33] },
//         height: 0,
//         timestamp: 0,
//     };
  
//     println!("Calling the Execute function");
//     let response = unsafe {
//         Execute(
//             &SimpleMutable { value: 0 },
//             &ctx, &execution_params)
//     };

//     println!("The response id is: {}", response.id);
// }

#[link(name = "simulator", kind = "dylib")]
extern "C" {
    fn TriggerCallback(cb: extern fn(i: i32));
}


#[no_mangle]
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

struct RustObject {
    a: i32

}


extern "C" fn callback(i : i32) {
    println!("Im called from C with value: {}", i);
}




fn main () {
    // let mut rust_object = Box::new(RustObject { a: 5 });
    unsafe {
        TriggerCallback(callback);
    }
}
