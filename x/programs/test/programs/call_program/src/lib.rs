use wasmlanche_sdk::ExternalCallError;
use wasmlanche_sdk::{public, types::Address, Context, Gas, Program};

#[public]
pub fn simple_call(_: Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(_: Context, target: Program, max_units: Gas) -> i64 {
    let res = target.call_function("simple_call", &[], max_units);
    borsh::from_slice::<Result<_, ExternalCallError>>(&res)
        .unwrap()
        .unwrap()
}

#[public]
pub fn actor_check(context: Context) -> Address {
    let Context { actor, .. } = context;
    actor
}

#[public]
pub fn actor_check_external(_: Context, target: Program, max_units: Gas) -> Address {
    let res = target.call_function("actor_check", &[], max_units);
    borsh::from_slice::<Result<_, ExternalCallError>>(&res)
        .expect("failure")
        .unwrap()
}

#[public]
pub fn call_with_param(_: Context, value: i64) -> i64 {
    value
}

#[public]
pub fn call_with_param_external(_: Context, target: Program, max_units: Gas, value: i64) -> i64 {
    let res = target.call_function("call_with_param", &value.to_le_bytes(), max_units);
    borsh::from_slice::<Result<_, ExternalCallError>>(&res)
        .unwrap()
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
    let args = borsh::to_vec(&(value1, value2)).unwrap();
    let res = target.call_function("call_with_two_params", &args, max_units);
    borsh::from_slice::<Result<_, ExternalCallError>>(&res)
        .unwrap()
        .unwrap()
}
