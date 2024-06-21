use wasmlanche_sdk::{public, types::Address, Context, Id};

#[public]
pub fn deploy(ctx: Context, program_id: Id) -> Address {
    let Context { program, .. } = ctx;
    program.deploy(program_id, &[]).unwrap()
}
