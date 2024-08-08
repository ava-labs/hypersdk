use wasmlanche_sdk::{public, Address, Context, Id};

#[public]
pub fn deploy(ctx: &mut Context, program_id: Id) -> Address {
    ctx.program().deploy(program_id, &[])
}
