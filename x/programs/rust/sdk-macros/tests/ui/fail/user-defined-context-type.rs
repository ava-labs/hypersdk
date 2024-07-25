use borsh::BorshDeserialize;
use sdk_macros::public;

#[derive(BorshDeserialize)]
struct Context;

#[public]
pub fn test(_: &mut Context) {}

fn main() {}
