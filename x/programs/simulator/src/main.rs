#![allow(non_upper_case_globals)]
#![allow(non_camel_case_types)]
#![allow(non_snake_case)]
// include the generated bindings
// reference: https://rust-lang.github.io/rust-bindgen/tutorial-3.html
include!(concat!(env!("OUT_DIR"), "/bindings.rs"));

mod simulator;
mod state;
mod types;
use simulator::Simulator;
use wasmlanche_sdk::Address as SdkAddress;

// fn main() {
//     // for now `simulator` is simple state. but later it will have additional fields + methods
//     let mut simulator = Simulator::new();
//     let gas = 100000000 as u64;
//     let actor = SdkAddress::default();
//     simulator.actor = actor;
//     let program_path = "/Users/sam.liokumovich/Documents/hypersdk/x/programs/rust/examples/token/build/wasm32-unknown-unknown/debug/token.wasm";

//     let program_response = simulator.create_program(program_path);
//     let program_address = program_response.program().unwrap();

//     simulator.execute(program_address, "init", (("Test"), ("TST")), gas);

//     let execute_response = simulator.execute(program_address, "name", (), gas);
//     let response = execute_response.result::<String>();
//     println!("Response : {:?}", response);

// let execute_response = simulator.execute(program_address, "get_value", ((actor),), gas);
// let response = execute_response.result::<u64>();

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
// }

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
    println!("Response : {:?}", response);

    // SimulatorCallContext::new();
}
