// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::hash::{DefaultHasher, Hasher};
use wasmlanche_sdk::{public, state_schema, Address, Context};

pub type Units = u64;

state_schema! {
    // Maps address to game balance
    Balance(Address) => Units
}

#[public]
// Returns whether the player has bust or not
pub fn hit(context: &mut Context, seed: u64) -> bool {
    let actor = context.actor();
    let curr_balance = context
        .get(Balance(actor))
        .expect("failed to get actor balance");

    let mut s = DefaultHasher::new();
    s.write_u64(seed);
    // True if even, false if odd
    // p = 0.5
    let bust = (s.finish() % 2) == 0;
    if bust {
        // Delete account
        context
            .delete(Balance(actor))
            .expect("failed to delete account");
    } else {
        // Double balance (or init to 1 if zero)
        let new_balance = match curr_balance {
            Some(v) => v * 2,
            None => 1,
        };

        context
            .store_by_key(Balance(actor), new_balance)
            .expect("failed to double actor balance");
    }

    bust
}

#[public]
pub fn get_balance(context: &mut Context, actor: Address) -> Units {
    context
        .get(Balance(actor))
        .expect("failed to get actor balance")
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
        simulator.set_actor(actor);
        let error = simulator.create_program(PROGRAM_PATH).has_error();
        assert!(!error, "Create program errored")
    }

    #[test]
    fn get_zero_balance() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);
        let gas = 100000000;
        let bob = Address::new([1; 33]);
        let blackjack_address = simulator.create_program(PROGRAM_PATH).program().unwrap();

        let value = simulator
            .call_program(blackjack_address, "get_balance", ((bob),), gas)
            .result::<u64>()
            .unwrap();

        assert_eq!(value, 0);
    }
}
