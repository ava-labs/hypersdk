// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use tutorial::Count;
use wasmlanche::{
    simulator::{Error, SimpleState, Simulator},
    Address,
};

// This is a constant that is set by the build script. It's the path to the
// .wasm file that's output when we compile.
const CONTRACT_PATH: &str = env!("CONTRACT_PATH");

// Let's not worry about how much gas things cost for now.
const LARGE_AMOUNT_OF_GAS: u64 = 100_000_000;

#[test]
fn create_contract() -> Result<(), Error> {
    let mut state = SimpleState::new();
    // The simulator needs mutable access to state.
    let simulator = Simulator::new(&mut state);

    simulator.create_contract(CONTRACT_PATH)?;

    Ok(())
}

#[test]
fn increment() {
    let bob = Address::new([1; 33]); // 1 repeated 33 times

    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    let counter_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<(), _>(counter_address, "inc", (bob, 10u64), LARGE_AMOUNT_OF_GAS)
        .unwrap();

    let value: Count = simulator
        .call_contract(counter_address, "get_value", bob, LARGE_AMOUNT_OF_GAS)
        .unwrap();

    assert_eq!(value, 10);
}

#[test]
fn inc_by_one() {
    let bob = Address::new([1; 33]);

    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);

    // the default actor is the 0-address instead of bob
    simulator.set_actor(bob);

    let counter_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<(), _>(counter_address, "inc_me_by_one", (), LARGE_AMOUNT_OF_GAS)
        .unwrap();

    let value: Count = simulator
        .call_contract(counter_address, "get_value", bob, LARGE_AMOUNT_OF_GAS)
        .unwrap();

    assert_eq!(value, 1);
}
