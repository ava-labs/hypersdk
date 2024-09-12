// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context, Gas};

#[public]
pub fn simple_call(_: &mut Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(ctx: &mut Context, target: Address, max_units: Gas) -> i64 {
    ctx.call_contract(target, "simple_call", &[], max_units, 0)
        .unwrap()
}

#[public]
pub fn actor_check(context: &mut Context) -> Address {
    context.actor()
}

#[public]
pub fn actor_check_external(ctx: &mut Context, target: Address, max_units: Gas) -> Address {
    ctx.call_contract(target, "actor_check", &[], max_units, 0)
        .expect("failure")
}

#[public]
pub fn call_with_param(_: &mut Context, value: i64) -> i64 {
    value
}

#[public]
pub fn call_with_param_external(
    ctx: &mut Context,
    target: Address,
    max_units: Gas,
    value: i64,
) -> i64 {
    ctx.call_contract(
        target,
        "call_with_param",
        &value.to_le_bytes(),
        max_units,
        0,
    )
    .unwrap()
}

#[public]
pub fn call_with_two_params(_: &mut Context, value1: i64, value2: i64) -> i64 {
    value1 + value2
}

#[public]
pub fn call_with_two_params_external(
    ctx: &mut Context,
    target: Address,
    max_units: Gas,
    value1: i64,
    value2: i64,
) -> i64 {
    let args: Vec<_> = value1
        .to_le_bytes()
        .into_iter()
        .chain(value2.to_le_bytes())
        .collect();
    ctx.call_contract(target, "call_with_two_params", &args, max_units, 0)
        .unwrap()
}
