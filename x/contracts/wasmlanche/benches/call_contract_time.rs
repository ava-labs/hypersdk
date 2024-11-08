// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use criterion::{criterion_group, criterion_main, BatchSize, Criterion};
use std::hint::black_box;
use wasmlanche::Address;
use wasmlanche_test::Builder;

mod contracts;
use contracts::{Contract, Nft};

criterion_group!(benches, call_contract, call_nft_mint);
criterion_main!(benches);

fn call_contract(c: &mut Criterion) {
    c.bench_function("call_contract", |b| {
        b.iter_batched(
            || Builder::new("test-crate"),
            |builder| {
                let mut contract = Contract::new(builder);
                contract.always_true();
                contract
            },
            BatchSize::PerIteration,
        )
    });
}

fn call_nft_mint(c: &mut Criterion) {
    c.bench_function("call_nft_mint", |b| {
        let mut iter = 0..;

        b.iter_batched(
            || (iter.next().unwrap(), black_box(Builder::new("nft"))),
            |(id, builder)| {
                let mut nft = Nft::new(builder);
                nft.mint(Address::new(black_box([2; 33])), black_box(id));
                nft
            },
            BatchSize::PerIteration,
        )
    });
}
