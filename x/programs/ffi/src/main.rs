use std::{collections::HashMap, ffi::CString};

use libc::{c_char, c_int, c_uchar, c_uint, c_void};

mod types;
mod state;
mod simulator;
use types::{ExecutionRequest, Response, SimpleMutable, SimulatorContext, ID, Address};
use state::{get_state_callback, insert_state_callback, remove_state_callback, GetStateCallback, Mutable, SimpleState};

#[link(name = "simulator", kind = "dylib")]
extern "C" {
    fn CallProgram(db: *mut Mutable);
}

fn main () {
    // for now `simulator` is simple state. but later it will have additional fields + methods
    let mut simulator = SimpleState::new();
    let state = Mutable {
        obj: &simulator as *const SimpleState as *mut SimpleState,
        get_state: get_state_callback,
        insert_state: insert_state_callback,
        remove_state: remove_state_callback,
    };
    unsafe {
        CallProgram(&state as *const Mutable as *mut Mutable);
    }
    // simulator.insert(vec![1, 2, 3], vec![4, 5, 6]);
    // unsafe {
    //     CallProgram(&state as *const Mutable as *mut Mutable);
    // }
}


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

