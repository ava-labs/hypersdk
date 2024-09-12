// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{
    simulator::{Error, SimpleState, Simulator},
    Address,
};

const CONTRACT_PATH: &str = env!("CONTRACT_PATH");

type Units = u64;

const MAX_UNITS: u64 = 1000000;
#[test]
fn create_contract() -> Result<(), Error> {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    simulator.create_contract(CONTRACT_PATH)?;

    Ok(())
}

#[test]
// initialize the token, check that the statekeys are set to the correct values
fn init_token() {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    let contract_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<(), _>(contract_address, "init", ("Test", "TST"), MAX_UNITS)
        .unwrap();

    let supply = simulator
        .call_contract::<Units, _>(contract_address, "total_supply", (), MAX_UNITS)
        .unwrap();
    assert_eq!(supply, 0);

    let symbol = simulator
        .call_contract::<String, _>(contract_address, "symbol", (), MAX_UNITS)
        .unwrap();
    assert_eq!(symbol, "TST");

    let name = simulator
        .call_contract::<String, _>(contract_address, "name", (), MAX_UNITS)
        .unwrap();
    assert_eq!(name, "Test");
}

#[test]
fn mint() {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    let alice = Address::new([1; 33]);
    let alice_initial_balance = 1000;

    let contract_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<(), _>(contract_address, "init", ("Test", "TST"), MAX_UNITS)
        .unwrap();

    simulator
        .call_contract::<(), _>(
            contract_address,
            "mint",
            (alice, alice_initial_balance),
            MAX_UNITS,
        )
        .unwrap();

    let balance: Units = simulator
        .call_contract(contract_address, "balance_of", (alice,), MAX_UNITS)
        .unwrap();
    assert_eq!(balance, alice_initial_balance);

    let total_supply: Units = simulator
        .call_contract(contract_address, "total_supply", (), MAX_UNITS)
        .unwrap();
    assert_eq!(total_supply, alice_initial_balance);
}

#[test]
fn burn() {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    let alice = Address::new([1; 33]);
    let alice_initial_balance = 1000;
    let alice_burn_amount = 100;

    let contract_address = simulator.create_contract(CONTRACT_PATH).unwrap().address;

    simulator
        .call_contract::<(), _>(contract_address, "init", ("Test", "TST"), MAX_UNITS)
        .unwrap();

    simulator
        .call_contract::<(), _>(
            contract_address,
            "mint",
            (alice, alice_initial_balance),
            MAX_UNITS,
        )
        .unwrap();

    simulator
        .call_contract::<Units, _>(
            contract_address,
            "burn",
            (alice, alice_burn_amount),
            MAX_UNITS,
        )
        .unwrap();

    let balance: Units = simulator
        .call_contract(contract_address, "balance_of", (alice,), MAX_UNITS)
        .unwrap();
    assert_eq!(balance, alice_initial_balance - alice_burn_amount);
}
