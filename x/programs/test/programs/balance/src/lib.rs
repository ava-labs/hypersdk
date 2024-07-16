use wasmlanche_sdk::{
    public, send, state::get_balance, types::Address, types::Gas, Context, Program,
};

#[public]
pub fn balance(ctx: Context) -> u64 {
    get_balance(ctx.actor())
}

#[public]
pub fn send_balance(_: Context, recipient: Address) -> bool {
    send(recipient, 1).is_ok()
}

#[public]
pub fn send_via_call(_: Context, target: Program, max_units: Gas, value: u64) -> u64 {
    target
        .call_function("balance", &[], max_units, value)
        .unwrap()
}
