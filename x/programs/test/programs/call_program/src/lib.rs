use wasmlanche_sdk::{
    public,
    types::{Address, Gas},
    Context, DeferDeserialize, Program,
};

#[public]
pub fn simple_call(_: Context) -> i64 {
    0
}

#[public]
pub fn simple_call_external(_: Context, target: Program, max_units: Gas) -> i64 {
    target.call_function("simple_call", &[], max_units).unwrap()
}

#[public]
pub fn actor_check(context: Context) -> Address {
    context.actor()
}

#[public]
pub fn actor_check_external(_: Context, target: Program, max_units: Gas) -> Address {
    target
        .call_function("actor_check", &[], max_units)
        .expect("failure")
}

#[public]
pub fn call_with_param(_: Context, value: i64) -> i64 {
    value
}

#[public]
pub fn call_with_param_external(_: Context, target: Program, max_units: Gas, value: i64) -> i64 {
    let params = borsh::to_vec(&value).expect("serialization failed");
    target
        .call_function("call_with_param", &params, max_units)
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
    let args: Vec<_> = borsh::to_vec(&(value1, value2)).expect("serialization failed");
    target
        .call_function("call_with_two_params", &args, max_units)
        .unwrap()
}

#[public]
pub fn call_deferred(_: Context, target: Program, max_units: Gas) -> i64 {
    let bytes = target
        .call_function::<DeferDeserialize>("simple_call", &[], max_units)
        .unwrap();
    bytes.deserialize()
}
