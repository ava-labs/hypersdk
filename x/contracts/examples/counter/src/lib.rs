// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, state_schema, Address, Context};

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
#[cfg(not(feature = "bindings"))]
mod tests {
    use super::*;

    #[test]
    fn initialized_value_is_zero() {
        let mut context = Context::new();
        let address = Address::default();

        let value = get_value(&mut context, address);
        assert_eq!(value, 0);
    }

    #[test]
    fn test_inc() {
        let mut context = Context::new();
        let address = Address::default();
        let amount = 5;

        let inc = inc(&mut context, address, amount);
        assert!(inc);

        let value = get_value(&mut context, address);
        assert_eq!(value, amount);
    }
}
