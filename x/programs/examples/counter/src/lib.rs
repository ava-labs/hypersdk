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
