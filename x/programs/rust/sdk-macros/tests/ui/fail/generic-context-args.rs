use wasmlanche_sdk::{borsh::BorshDeserialize, public};

#[derive(BorshDeserialize)]
#[borsh(crate = "wasmlanche_sdk::borsh")]
pub struct Context<T>(T);

#[public]
pub fn always_true(_: &mut Context<u8>) -> bool {
    true
}

fn main() {}
