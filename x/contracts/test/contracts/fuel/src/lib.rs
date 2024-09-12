// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context, ExternalCallError};

#[public]
pub fn get_fuel(ctx: &mut Context) -> u64 {
    ctx.remaining_fuel()
}

#[public]
pub fn out_of_fuel(ctx: &mut Context, target: Address) -> ExternalCallError {
    ctx.call_contract::<u64>(target, "get_fuel", &[], 0, 0)
        .unwrap_err()
}
