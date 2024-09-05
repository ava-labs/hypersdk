// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context, ExternalCallContext};

#[public]
pub fn inc(_: &mut Context, external: Address, of: Address) {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0).into();
    counter::inc(&ctx, of, 1);
}

#[public]
pub fn get_value(_: &mut Context, external: Address, of: Address) -> u64 {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0).into();
    counter::get_value(&ctx, of)
}
