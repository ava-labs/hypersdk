use wasmlanche_sdk::{public, Context};

#[public]
pub fn always_true(_: &mut Context, _: bool) -> bool {
    true
}

fn main() {}
