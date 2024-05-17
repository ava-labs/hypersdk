use borsh::BorshSerialize;
use wasmlanche_sdk::{public, Context, Program};

#[derive(BorshSerialize)]
pub struct ComplexReturn {
    program: Program,
    max_units: i64,
}

#[public]
pub fn get_value(ctx: Context) -> ComplexReturn {
    let Context {program, ..} = ctx;
    ComplexReturn{program:program, max_units:1000}
}