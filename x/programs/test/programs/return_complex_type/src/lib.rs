use wasmlanche_sdk::{borsh::BorshSerialize, public, Address, Context, Gas};

#[derive(BorshSerialize)]
#[borsh(crate = "wasmlanche_sdk::borsh")]
pub struct ComplexReturn {
    account: Address,
    max_units: Gas,
}

#[public]
pub fn get_value(ctx: &mut Context) -> ComplexReturn {
    let account = *ctx.program().account();
    ComplexReturn {
        account,
        max_units: 1000,
    }
}
