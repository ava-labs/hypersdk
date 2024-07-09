use wasmlanche_sdk::{public, send, state::get_balance, types::Address, Context};

#[public]
pub fn balance(ctx: Context) -> u64 {
    get_balance(ctx.actor())
}

#[public]
pub fn send_balance(_: Context, recipient: Address) -> bool {
    send(recipient, 1).is_ok()
}
