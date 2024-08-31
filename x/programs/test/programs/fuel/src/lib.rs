// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Context, ExternalCallError, Program};

#[public]
pub fn get_fuel(ctx: &mut Context) -> u64 {
    ctx.remaining_fuel()
}

#[public]
pub fn out_of_fuel(_: &mut Context, target: Program) -> ExternalCallError {
    target
        .call_function::<u64>("get_fuel", &[], 0, 0)
        .unwrap_err()
}
