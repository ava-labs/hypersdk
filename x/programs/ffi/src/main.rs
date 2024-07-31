use std::{collections::HashMap, ffi::CString};

use borsh::BorshSerialize;
use libc::{c_char, c_int, c_uchar, c_uint, c_void};

mod simulator;
mod state;
mod types;
use simulator::Simulator;
use types::{Address, ExecutionRequest, SimulatorContext};
use wasmlanche_sdk::Address as SdkAddress;

fn main() {
    // for now `simulator` is simple state. but later it will have additional fields + methods
    let simulator = Simulator::new();
    simulator.call_program_test();
    let program_path = "/Users/sam.liokumovich/Documents/hypersdk/x/programs/rust/examples/token/build/wasm32-unknown-unknown/debug/token.wasm";

    let program_response = simulator.create_program(program_path);

    let method = CString::new("init").expect("CString::new failed");
    let items = vec!["Test", "TST"];
    let params: Vec<u8> = serialize_and_concat(&items);
    let param_length = params.len() as c_uint;
    let max_gas = 100000000;

    let execution_params = ExecutionRequest {
        method: method.as_ptr(),
        params: params.as_ptr(),
        param_length,
        max_gas,
    };

    let context = SimulatorContext {
        program_address: program_response.program_c_address().unwrap(),
        actor_address: Address { address: [0; 33] },
        height: 0,
        timestamp: 0,
    };
    
    let execute_response = simulator.execute(&context, &execution_params);

    let method = CString::new("symbol").expect("CString::new failed");
    let execution_params = ExecutionRequest {
        method: method.as_ptr(),
        // TODO: maybe make params null here and check for null pointers in ffi?
        params: std::ptr::null(),
        param_length,
        max_gas,
    };
    let execute_response = simulator.execute(&context, &execution_params);

}

fn serialize_and_concat<T: BorshSerialize>(items: &[T]) -> Vec<u8> {
    let mut result = Vec::new();
    for item in items {
        let serialized = borsh::to_vec(item).expect("Serialization failed");
        result.extend(serialized);
    }
    result
}


// #[link(name = "simulator", kind = "dylib")]
// extern "C" {
//     fn Execute(db: *const SimpleMutable, ctx: *const SimulatorContext, request: *const ExecutionRequest) -> Response;
// }

// fn main() {
//    
//     println!("Calling the Execute function");
//     let response = unsafe {
//         Execute(
//             &SimpleMutable { value: 0 },
//             &ctx, &execution_params)
//     };

//     println!("The response id is: {}", response.id);
// }
