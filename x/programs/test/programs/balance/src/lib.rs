use wasmlanche_sdk::{
    balance::{get, send},
    public,
    types::Address,
    Context,
};

#[public]
pub fn balance(ctx: Context) -> u64 {
    get(ctx.actor())
}

#[public]
pub fn send_balance(_: Context, recipient: Address) -> bool {
    send(recipient, 1).is_ok()
}
