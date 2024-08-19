// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche_sdk::{borsh::BorshSerialize, public, Address, Context};

#[derive(BorshSerialize)]
#[borsh(crate = "wasmlanche_sdk::borsh")]
pub struct ComplexReturn {
    account: Address,
    max_units: u64,
}

#[public]
pub fn get_value(ctx: &mut Context) -> ComplexReturn {
    let account = *ctx.program().account();
    ComplexReturn {
        account,
        max_units: 1000,
    }
}
