// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche_sdk::{get_balance, public, send, Address, Context, Gas, Program};

#[public]
pub fn balance(ctx: &mut Context) -> u64 {
    get_balance(ctx.actor())
}

#[public]
pub fn send_balance(_: &mut Context, recipient: Address) -> bool {
    send(recipient, 1).is_ok()
}

#[public]
pub fn send_via_call(_: &mut Context, target: Program, max_units: Gas, value: u64) -> u64 {
    target
        .call_function("balance", &[], &max_units, value)
        .unwrap()
}
