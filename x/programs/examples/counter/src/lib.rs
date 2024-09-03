// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#[cfg(not(feature = "bindings"))]
use wasmlanche::Context;
use wasmlanche::{public, state_schema, Address};

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
    use wasmlanche::{Address, Context};

    use crate::*;

    #[test]
    fn increment() {
        let mut context = Context::new_test_context();
        let bob = Address::new([1; 33]);

        let result = inc(&mut context, bob, 10);
        assert!(result, "increment failed");

        let value = get_value(&mut context, bob);
        assert_eq!(value, 10);
    }

    #[test]
    fn inc_already_set() {
        let mut context = Context::new_test_context();
        let bob = Address::new([1; 33]);
        context.store_by_key(Counter(bob), 10_u64).unwrap();

        let result = inc(&mut context, bob, 10);
        assert!(result, "increment failed");

        let value = get_value(&mut context, bob);
        assert_eq!(value, 20);
    }
}
