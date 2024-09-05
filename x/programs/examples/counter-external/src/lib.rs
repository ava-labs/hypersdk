// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context};

#[public]
pub fn inc(ctx: &mut Context, external: Address, of: Address) {
    let ctx = ctx.new_external_call_context(external, 1_000_000, 0);
    counter::inc(&ctx, of, 1);
}

#[public]
pub fn get_value(ctx: &mut Context, external: Address, of: Address) -> u64 {
    let ctx = ctx.new_external_call_context(external, 1_000_000, 0);
    counter::get_value(&ctx, of)
}
