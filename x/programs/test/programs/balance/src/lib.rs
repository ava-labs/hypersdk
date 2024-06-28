use wasmlanche_sdk::{
    balance::{get_balance, send},
    public,
    types::Address,
    Context,
};

#[public]
pub fn balance(ctx: Context) -> u64 {
    get_balance(ctx.actor())
}

#[public]
pub fn send_balance(_: Context, recipient: Address) -> bool {
    match send(recipient, 1) {
        Ok(_) => true,
        Err(_) => false,
    }
}
