// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context, ExternalCallError};

#[public]
pub fn get_fuel(ctx: &mut Context) -> u64 {
    ctx.remaining_fuel()
}

#[public]
pub fn out_of_fuel(ctx: &mut Context, target: Address) -> ExternalCallError {
    ctx.call_contract_builder(target)
        .with_max_units(0)
        .call_function::<u64>("get_fuel", &[])
        .unwrap_err()
}
