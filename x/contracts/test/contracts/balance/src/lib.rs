// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context};

#[public]
pub fn balance(ctx: &mut Context) -> u64 {
    ctx.get_balance(ctx.actor())
}

#[public]
pub fn send_balance(ctx: &mut Context, recipient: Address) -> bool {
    ctx.send(recipient, 1).is_ok()
}

#[public]
pub fn send_via_call(ctx: &mut Context, target: Address, max_units: u64, value: u64) -> u64 {
    ctx.call_contract_builder(target)
        .with_max_units(max_units)
        .with_value(value)
        .call_function("balance", &[])
        .unwrap()
}
