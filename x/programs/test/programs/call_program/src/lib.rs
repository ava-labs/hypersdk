use wasmlanche_sdk::{
    public,
    types::{Address, Gas},
    Context, Program,
};

#[public]
pub fn simple_call(_: Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(_: Context, target: Program, max_units: Gas) -> i64 {
    target
        .call_function("simple_call", &[], max_units, 0)
        .unwrap()
}

#[public]
pub fn actor_check(context: Context) -> Address {
    context.actor()
}

#[public]
pub fn actor_check_external(_: Context, target: Program, max_units: Gas) -> Address {
    target
        .call_function("actor_check", &[], max_units, 0)
        .expect("failure")
}

#[public]
pub fn call_with_param(_: Context, value: i64) -> i64 {
    value
}

#[public]
pub fn call_with_param_external(_: Context, target: Program, max_units: Gas, value: i64) -> i64 {
    target
        .call_function("call_with_param", &value.to_le_bytes(), max_units, 0)
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
    let args: Vec<_> = value1
        .to_le_bytes()
        .into_iter()
        .chain(value2.to_le_bytes())
        .collect();
    target
        .call_function("call_with_two_params", &args, max_units, 0)
        .unwrap()
}
