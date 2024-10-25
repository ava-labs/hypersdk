// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::simulator::{Error, SimpleState, Simulator};

const CONTRACT_PATH: &str = env!("CONTRACT_PATH");

#[test]
fn create_contract() -> Result<(), Error> {
    let mut state = SimpleState::new();
    let simulator = Simulator::new(&mut state);

    simulator.create_contract(CONTRACT_PATH)?;

    Ok(())
}
