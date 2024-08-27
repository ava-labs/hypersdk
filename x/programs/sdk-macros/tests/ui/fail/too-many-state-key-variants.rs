// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use seq_macro::seq;
use wasmlanche::state_schema;

seq!(N in 0..=255 {
    state_schema! {
        #(
            Variant~N => u8,
        )*
        Variant256 => u8,
    }
});

fn main() {}
