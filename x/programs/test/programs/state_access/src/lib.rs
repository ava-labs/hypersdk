// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, state_schema, Context};

state_schema! {
    State => i64,
}

/// Initializes the contract with a name, symbol, and total supply.
#[public]
pub fn put(context: &mut Context, value: i64) {
    context
        .store_by_key(State, value)
        .expect("failed to store state");
}

#[public]
pub fn get(context: &mut Context) -> Option<i64> {
    context.get(State).expect("failed to get state")
}

#[public]
pub fn delete(context: &mut Context) -> Option<i64> {
    context.delete(State).expect("failed to get state")
}
