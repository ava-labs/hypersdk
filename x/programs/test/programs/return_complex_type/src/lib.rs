use borsh::BorshSerialize;
use wasmlanche_sdk::{public, types::Gas, Context, Program};

#[derive(BorshSerialize)]
pub struct ComplexReturn {
    program: Program,
    max_units: Gas,
}

#[public]
pub fn get_value(ctx: Context) -> ComplexReturn {
    let Context { program, .. } = ctx;
    ComplexReturn {
        program,
        max_units: 1000,
    }
}
