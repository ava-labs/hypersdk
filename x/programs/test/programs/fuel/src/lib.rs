use wasmlanche_sdk::{public, Context, ExternalCallError, Program};

#[public]
pub fn get_fuel(ctx: Context) -> u64 {
    ctx.program().remaining_fuel()
}

#[public]
pub fn out_of_fuel(_: Context, target: Program) -> ExternalCallError {
    target.call_function::<u64>("get_fuel", &[], 0).unwrap_err()
}
