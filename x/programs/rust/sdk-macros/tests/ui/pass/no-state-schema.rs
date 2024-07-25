use wasmlanche_sdk::{public, Context};

#[public]
pub fn do_something<'a>(_: &'a mut Context) -> u8 {
    0
}

fn main() {}
