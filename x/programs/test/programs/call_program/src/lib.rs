// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche_sdk::{public, Address, Context, GasUnits, Program};

#[public]
pub fn simple_call(_: &mut Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(_: &mut Context, target: Program, max_units: GasUnits) -> i64 {
    target
        .call_function("simple_call", &[], &max_units, 0)
        .unwrap()
}

#[public]
pub fn actor_check(context: &mut Context) -> Address {
    context.actor()
}

#[public]
pub fn actor_check_external(_: &mut Context, target: Program, max_units: GasUnits) -> Address {
    target
        .call_function("actor_check", &[], &max_units, 0)
        .expect("failure")
}

#[public]
pub fn call_with_param(_: &mut Context, value: i64) -> i64 {
    value
}

#[public]
pub fn call_with_param_external(
    _: &mut Context,
    target: Program,
    max_units: GasUnits,
    value: i64,
) -> i64 {
    target
        .call_function("call_with_param", &value.to_le_bytes(), &max_units, 0)
        .unwrap()
}

#[public]
pub fn call_with_two_params(_: &mut Context, value1: i64, value2: i64) -> i64 {
    value1 + value2
}

#[public]
pub fn call_with_two_params_external(
    _: &mut Context,
    target: Program,
    max_units: GasUnits,
    value1: i64,
    value2: i64,
) -> i64 {
    let args: Vec<_> = value1
        .to_le_bytes()
        .into_iter()
        .chain(value2.to_le_bytes())
        .collect();
    target
        .call_function("call_with_two_params", &args, &max_units, 0)
        .unwrap()
}
