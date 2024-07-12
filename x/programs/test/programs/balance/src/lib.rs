use wasmlanche_sdk::{
    balance::{get, send},
    public,
    types::Address,
    types::Gas,
    Context, Program,
};

#[public]
pub fn balance(ctx: Context) -> u64 {
    get(ctx.actor())
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
