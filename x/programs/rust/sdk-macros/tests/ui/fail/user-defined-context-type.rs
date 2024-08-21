// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use sdk_macros::public;
use wasmlanche_sdk::borsh::BorshDeserialize;

#[derive(BorshDeserialize)]
#[borsh(crate = "wasmlanche_sdk::borsh")]
struct Context;

#[public]
pub fn test(_: &mut Context) {}

fn main() {}
