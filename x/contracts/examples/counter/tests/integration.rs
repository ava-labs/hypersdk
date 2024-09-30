// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{
    simulator::{Error, SimpleState, Simulator},
    Address,
};
const CONTRACT_PATH: &str = env!("CONTRACT_PATH");

#[test]
fn init_contract() -> Result<(), Error> {
    let mut state = SimpleState::new();
    let mut simulator = Simulator::new(&mut state);

    let actor = Address::default();
    simulator.set_actor(actor);
    simulator.create_contract(CONTRACT_PATH)?;
    Ok(())
}

#[test]
fn increment() {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);
    let gas = 100000000;
    let bob = Address::new([1; 33]);
    let counter_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<bool, _>(counter_address, "inc", (bob, 10u64), gas)
        .unwrap();
    let value: u64 = simulator
        .call_contract(counter_address, "get_value", ((bob),), gas)
        .unwrap();

    assert_eq!(value, 10);
}
