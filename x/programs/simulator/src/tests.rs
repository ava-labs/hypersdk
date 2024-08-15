// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#[cfg(test)]
mod tests {
    use wasmlanche_sdk::Address;

    use crate::simulator::Simulator;
    use crate::state::SimpleState;

    #[test]
    fn set_balance() {
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let alice = Address::new([1; 33]);

        let bal = simulator.get_balance(alice);
        assert_eq!(bal, 0);

        simulator.set_balance(alice, 100);
        let bal = simulator.get_balance(alice);
        assert_eq!(bal, 100);
    }
}
