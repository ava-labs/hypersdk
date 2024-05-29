use wasmlanche_sdk::{public, Context};

#[public]
pub fn get_fuel(ctx: Context) -> u64 {
    let Context { program, .. } = ctx;
    program.remaining_fuel()
}
