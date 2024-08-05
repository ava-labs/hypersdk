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

fn main() {
    // for now `simulator` is simple state. but later it will have additional fields + methods
    let mut simulator = Simulator::new();
    let gas = 100000000_u64;
    let actor = SdkAddress::default();
    simulator.actor = actor;
    let program_path = "/Users/sam.liokumovich/Documents/hypersdk/x/programs/rust/examples/counter/build/wasm32-unknown-unknown/debug/counter.wasm";

    let program_response = simulator.create_program(program_path);
    let program_address = program_response.program().unwrap();

    let execute_response = simulator.call_program(program_address, "get_value", ((actor),), gas);
    let response = execute_response.result::<u64>();
    println!("Response : {:?}", response);

    let execute_response = simulator.call_program(program_address, "inc", ((actor), 10u64), gas);
    let response = execute_response.result::<bool>();
    println!("Response : {:?}", response);

    let execute_response = simulator.call_program(program_address, "get_value", ((actor),), gas);
    let response = execute_response.result::<u64>();
    println!("Response : {:?}", response);
}
