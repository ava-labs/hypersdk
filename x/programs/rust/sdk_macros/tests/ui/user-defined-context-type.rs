use borsh::BorshDeserialize;
use sdk_macros::public;

#[derive(BorshDeserialize)]
struct Context;

#[public]
pub fn test(_: Context) {}

fn main() {}
