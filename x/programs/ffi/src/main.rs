use std::{collections::HashMap, ffi::CString};

use libc::{c_char, c_int, c_uchar, c_uint, c_void};

mod types;
mod state;
mod simulator;
use types::{ExecutionRequest, Response, SimpleMutable, SimulatorContext, ID, Address};
use simulator::{get_state_callback, GetStateCallback, Simulator};
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
    fn CallProgram(cb: GetStateCallback, callData: *mut Simulator);
}

fn main () {
    let obj = Simulator::new();
    unsafe {
        CallProgram(get_state_callback, &obj as *const Simulator as *mut Simulator);
    }
}
