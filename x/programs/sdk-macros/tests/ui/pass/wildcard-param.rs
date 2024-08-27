// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Context};

#[public]
pub fn always_true(_: &mut Context, _: bool) -> bool {
    true
}

fn main() {}
