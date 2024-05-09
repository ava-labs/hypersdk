#![no_std]

use wasmlanche_sdk::{
    params::{self, Param},
    public, Context, Params, ProgramError,
};

#[public]
pub fn always_true(_: Context) -> i32 {
    true as i32
}

#[public]
pub fn combine_last_bit_of_each_id_byte(context: Context) -> u32 {
    context
        .program
        .id()
        .iter()
        .map(|byte| *byte as u32)
        .fold(0, |acc, byte| (acc << 1) + (byte & 1))
}

#[public]
pub fn faillible_fn(ctx: Context) -> i32 {
    // true as i32

    ctx.program
        .call_function("failing_fn", &Params::default(), 100000)
        .is_err() as i32
}

#[public]
pub fn failing_fn(_: Context) -> Result<i64, ProgramError> {
    Err(ProgramError::Custom("this is an error".into()))
}
