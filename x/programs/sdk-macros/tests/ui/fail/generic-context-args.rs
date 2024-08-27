// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche_sdk::{borsh::BorshDeserialize, public};

#[derive(BorshDeserialize)]
#[borsh(crate = "wasmlanche_sdk::borsh")]
pub struct Context<T>(T);

#[public]
pub fn always_true(_: &mut Context<u8>) -> bool {
    true
}

fn main() {}
