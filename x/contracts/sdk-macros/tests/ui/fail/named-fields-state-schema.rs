// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::state_schema;

state_schema! {
    Valid => u8,
    OtherValid(u8) => u8,
    Invalid { param: u8 } => u8,
}

fn main() {}
