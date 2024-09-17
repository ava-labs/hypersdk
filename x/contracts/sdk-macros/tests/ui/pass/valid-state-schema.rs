// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::state_schema;

state_schema! {
    Key => u64,
    Tuple(u8, u8) => u8,
    TupleReturn => (u8, u8),
    TupleKeyAndReturn(u8, u8) => (u8, u8),
}

fn main() {}
