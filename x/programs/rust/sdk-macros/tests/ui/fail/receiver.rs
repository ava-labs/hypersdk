// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use sdk_macros::public;

struct Foo;

impl Foo {
    #[public]
    pub fn test(&self) {}
}

fn main() {}
