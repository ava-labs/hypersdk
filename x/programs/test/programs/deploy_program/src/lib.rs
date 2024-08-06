use wasmlanche_sdk::{public, Address, Context};

#[public]
pub fn deploy(ctx: &mut Context, program_id: Vec<u8>) -> Address {
    ctx.program().deploy(&program_id, &[])
}
