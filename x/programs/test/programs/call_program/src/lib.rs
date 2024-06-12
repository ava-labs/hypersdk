use wasmlanche_sdk::{public, types::Address, Context, Gas, Program};

#[public]
pub fn simple_call(_: Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(_: Context, target: Program, max_units: Gas) -> i64 {
    target.call_function("simple_call", (), max_units).unwrap()
}

#[public]
pub fn actor_check(context: Context) -> Address {
    let Context { actor, .. } = context;
    actor
}

#[public]
pub fn actor_check_external(_: Context, target: Program, max_units: Gas) -> Address {
    target
        .call_function("actor_check", (), max_units)
        .expect("failure")
}

#[public]
pub fn call_with_param(_: Context, value: i64) -> i64 {
    value
}

#[public]
pub fn call_with_param_external(_: Context, target: Program, max_units: Gas, value: i64) -> i64 {
    target
        .call_function("call_with_param", value, max_units)
        .unwrap()
}

#[public]
pub fn call_with_two_params(_: Context, value1: i64, value2: i64) -> i64 {
    value1 + value2
}

#[public]
pub fn call_with_two_params_external(
    _: Context,
    target: Program,
    max_units: Gas,
    value1: i64,
    value2: i64,
) -> i64 {
    target
        .call_function("call_with_two_params", (value1, value2), max_units)
        .unwrap()
}
