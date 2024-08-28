// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Context};

#[public]
pub fn do_something<'a>(_: &'a mut Context) -> u8 {
    0
}

fn main() {}
