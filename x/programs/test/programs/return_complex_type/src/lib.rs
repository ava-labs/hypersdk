// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{borsh::BorshSerialize, public, Address, Context, Gas};

#[derive(BorshSerialize)]
#[borsh(crate = "wasmlanche::borsh")]
pub struct ComplexReturn {
    account: Address,
    max_units: Gas,
}

#[public]
pub fn get_value(ctx: &mut Context) -> ComplexReturn {
    let account = ctx.contract_address();
    ComplexReturn {
        account,
        max_units: 1000,
    }
}
