// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use contracts::{Contract, Nft};
use std::hint::black_box;
use wasmlanche::Address;
use wasmlanche_test::Builder;

mod contracts;

iai::main!(call_contract, call_nft_mint);

fn call_contract() {
    let builder = Builder::new("test-crate");
    Contract::new(builder).always_true();
}

fn call_nft_mint() {
    let builder = Builder::new("nft");
    Nft::new(builder).mint(Address::new(black_box([2; 33])), black_box(0));
}
