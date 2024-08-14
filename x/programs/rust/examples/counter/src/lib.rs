// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_schema, Address};

type Count = u64;

state_schema! {
    /// Counter for each address.
    Counter(Address) => Count,
}

/// Increments the count at the address by the amount.
#[public]
pub fn inc(context: &mut Context, to: Address, amount: Count) -> bool {
    let counter = amount + get_value(context, to);

    context
        .store_by_key(Counter(to), counter)
        .expect("serialization failed");

    true
}

/// Gets the count at the address.
#[public]
pub fn get_value(context: &mut Context, of: Address) -> Count {
    context
        .get(Counter(of))
        .expect("state corrupt")
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use simulator::{SimpleState, Simulator};
    use wasmlanche_sdk::Address;
    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);

        let actor = Address::default();
        simulator.actor = actor;
        let error = simulator.create_program(PROGRAM_PATH).has_error();
        assert!(!error, "Create program errored")
    }

    #[test]
    fn increment() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);
        let gas = 100000000;
        let bob = Address::new([1; 33]);
        let counter_address = simulator.create_program(PROGRAM_PATH).program().unwrap();
        simulator.call_program(counter_address, "inc", (bob, 10u64), gas);
        let value = simulator
            .call_program(counter_address, "get_value", ((bob),), gas)
            .result::<u64>()
            .unwrap();

        assert_eq!(value, 10);
    }
}
