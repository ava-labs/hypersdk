// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche_sdk::{public, Address, Context, Id};

#[public]
pub fn deploy(ctx: &mut Context, program_id: Id) -> Address {
    ctx.deploy(program_id, &[])
}
