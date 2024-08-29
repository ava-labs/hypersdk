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

/// Gets the count at the address.
#[public]
pub fn get_value(context: &mut Context, of: Address) -> Count {
    context
        .get(Counter(of))
        .expect("state corrupt")
        .unwrap_or_default()
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

#[cfg(test)]
mod tests {
    use simulator::{SimpleState, Simulator};
    use wasmlanche_sdk::{Address, Context};

    use crate::{get_value, inc};
    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);

        let actor = Address::default();
        simulator.set_actor(actor);
        let error = simulator.create_program(PROGRAM_PATH).has_error();
        assert!(!error, "Create program errored")
    }

    #[test]
    fn increment() {

        let mut context = Context::new_test_context();
        let bob = Address::new([1; 33]);
        let val = get_value(&mut context, bob);
        assert_eq!(val, 0);
        
        inc(&mut context, bob, 10);

        let val = get_value(&mut context, bob);
        assert_eq!(val, 10);
    }
}
