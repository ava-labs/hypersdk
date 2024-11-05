// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::Address;
use wasmlanche_test::{Builder, TestCrate, UserDefinedFn};

pub struct Contract {
    inner: TestCrate,
    always_true: UserDefinedFn,
}

impl Contract {
    #[inline]
    pub fn new(builder: Builder) -> Self {
        let mut inner = builder.build();
        let always_true = inner.get_user_defined_typed_func("always_true");

        Self { inner, always_true }
    }

    #[inline]
    pub fn always_true(&mut self) -> bool {
        let Self { always_true, inner } = self;
        let ctx = inner.allocate_context();

        always_true
            .call(inner.store_mut(), ctx)
            .expect("failed to call `always_true` function");

        let result = inner
            .store_mut()
            .data_mut()
            .take_result()
            .expect("always_true should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}

pub struct Nft {
    inner: TestCrate,
    mint: UserDefinedFn,
}

impl Nft {
    #[inline]
    pub fn new(builder: Builder) -> Self {
        let mut inner = builder.build();
        let mint = inner.get_user_defined_typed_func("mint");

        Self { inner, mint }
    }

    #[inline]
    pub fn mint(&mut self, address: Address, id: u64) {
        let Self { mint, inner } = self;

        let params = inner.allocate_params(&(address, id));

        mint.call(inner.store_mut(), params)
            .expect("failed to call `mint` function");

        let result = inner
            .store_mut()
            .data_mut()
            .take_result()
            .expect("mint should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}
