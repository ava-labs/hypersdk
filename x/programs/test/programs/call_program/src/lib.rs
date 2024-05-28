use wasmlanche_sdk::{public, Context, Program};

#[public]
pub fn get_value(_: Context) -> i64 {
    0
}

#[public]
pub fn get_value_external(_: Context, target: Program, max_units: i64) -> i64 {
    target
        .call_function("get_value", (), max_units)
        .unwrap()
}
