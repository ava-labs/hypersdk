use wasmlanche_sdk::{params::Param, public, Context, Program};

#[public]
pub fn get_value(_: Context) -> i64 {
    0
}

#[public]
pub fn get_value_external(_: Context, target: Program, max_units: i64) -> i64 {
    let v: Vec<Param> = vec![];
    target
        .call_function("get_value", &v.into_iter().collect(), max_units)
        .unwrap()
}
