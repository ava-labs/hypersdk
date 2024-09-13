// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#![deny(clippy::pedantic)]
use wasmlanche::{
    simulator::{SimpleState, Simulator},
    Address,
};

const CONTRACT_PATH: &str = env!("CONTRACT_PATH");

#[test]
fn inc_and_get_value() {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    let counter_path = CONTRACT_PATH
        .replace("counter-external", "counter")
        .replace("counter_external", "counter");

    let owner = Address::new([1; 33]);

    let counter_external = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    let counter = simulator.create_contract(&counter_path).unwrap().address;

    simulator
        .call_contract::<(), _>(counter_external, "inc", (counter, owner), 100_000_000)
        .unwrap();

    let response: u64 = simulator
        .call_contract(counter_external, "get_value", (counter, owner), 100_000_000)
        .unwrap();

    assert_eq!(response, 1);
}
