// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Context, Program, ProgramId};

#[public]
pub fn deploy(ctx: &mut Context, program_id: ProgramId) -> Program {
    ctx.deploy(program_id, &[])
}
