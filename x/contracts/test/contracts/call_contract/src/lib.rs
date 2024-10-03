// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context};

#[public]
pub fn simple_call(_: &mut Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(ctx: &mut Context, target: Address, max_units: u64) -> i64 {
    ctx.call_contract_builder(target)
        .with_max_units(max_units)
        .call_function("simple_call", &[])
        .unwrap()
}

#[public]
pub fn actor_check(context: &mut Context) -> Address {
    context.actor()
}

#[public]
pub fn actor_check_external(ctx: &mut Context, target: Address, max_units: u64) -> Address {
    ctx.call_contract_builder(target)
        .with_max_units(max_units)
        .call_function("actor_check", &[])
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
    max_units: u64,
    value: i64,
) -> i64 {
    ctx.call_contract_builder(target)
        .with_max_units(max_units)
        .call_function("call_with_param", &value.to_le_bytes())
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
    max_units: u64,
    value1: i64,
    value2: i64,
) -> i64 {
    let args: Vec<_> = value1
        .to_le_bytes()
        .into_iter()
        .chain(value2.to_le_bytes())
        .collect();
    ctx.call_contract_builder(target)
        .with_max_units(max_units)
        .call_function("call_with_two_params", &args)
        .unwrap()
}
