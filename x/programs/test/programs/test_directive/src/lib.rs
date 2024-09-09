// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#[cfg(test)]
mod tests {
    use wasmlanche::{Address, Context};

    #[test]
    fn test_balance() {
        let mut context = Context::new();
        let address = Address::default();
        let amount: u64 = 100;

        // set the balance
        context.mock_set_balance(address, amount);

        let balance = context.get_balance(address);
        assert_eq!(balance, amount);
    }
}
