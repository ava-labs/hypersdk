use wasmlanche_sdk::{public, state_keys, Context};

#[state_keys]
pub enum NoVariants {}

#[public]
pub fn do_something(_: Context<NoVariants>) -> u8 {
    0
}

fn main() {}
