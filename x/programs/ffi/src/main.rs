use std::{collections::HashMap, ffi::CString};

use borsh::BorshSerialize;
use libc::{c_char, c_int, c_uchar, c_uint, c_void};

mod simulator;
mod state;
mod types;
use simulator::Simulator;
use types::{Address, ExecutionRequest, SimulatorCallContext};
use wasmlanche_sdk::Address as SdkAddress;

fn main() {
    // for now `simulator` is simple state. but later it will have additional fields + methods
    let mut simulator = Simulator::new();
    let gas = 100000000 as u64;
    let actor = SdkAddress::default();
    simulator.actor = actor;
    let program_path = "/Users/sam.liokumovich/Documents/hypersdk/x/programs/rust/examples/counter/build/wasm32-unknown-unknown/debug/counter.wasm";

    let program_response = simulator.create_program(program_path);
    let program_address = program_response.program().unwrap();

    let execute_response = simulator.execute(program_address, "get_value", ((actor),), gas);
    let response = execute_response.result::<u64>();
    println!("Response : {:?}", response);

    let execute_response = simulator.execute(program_address, "inc", ((actor), 10u64), gas);
    let response = execute_response.result::<bool>();
    println!("Response : {:?}", response);

    let execute_response = simulator.execute(program_address, "get_value", ((actor),), gas);
    let response = execute_response.result::<u64>();

    // let params = ("Test", "TST");

    // // let params: Vec<u8> = serialize_and_concat(&items);
    // let max_gas = 100000000;
    // let execute_response = simulator.execute(program_address, "init", params, max_gas);
    // println!("execution response {:?}", execute_response);
    // let execute_response = simulator.execute(program_address, "name", (), max_gas);

    // simulator.call_program_test();

    // let a = SdkAddress::new([1; 33]);
    // let items = vec![a];
    // let params = serialize_and_concat(&items);
    // let param_length = params.len() as c_uint;
    // let method = CString::new("balance_of").expect("CString::new failed");
    // let execution_params = ExecutionRequest {
    //     method: method.as_ptr(),
    //     // TODO: maybe make params null here and check for null pointers in ffi?
    //     params: params.as_ptr(),
    //     param_length,
    //     max_gas,
    // };
    // let execute_response = simulator.execute(&context, &execution_params);
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
//     fn Execute(db: *const SimpleMutable, ctx: *const SimulatorCallContext, request: *const ExecutionRequest) -> Response;
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
