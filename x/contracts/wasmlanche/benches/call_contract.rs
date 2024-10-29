use criterion::{criterion_group, criterion_main, BatchSize, Criterion};
use wasmlanche_test::{Builder, TestCrate, UserDefinedFn};

criterion_group!(benches, call_contract);
criterion_main!(benches);

fn call_contract(c: &mut Criterion) {
    c.bench_function("call_contract", |b| {
        b.iter_batched(
            || Contract::new(),
            |mut contract| contract.always_true(),
            BatchSize::PerIteration,
        )
    });
}

struct Contract {
    inner: TestCrate,
    always_true: UserDefinedFn,
}

impl Contract {
    fn new() -> Self {
        let mut inner = Builder::new("bench-crate").build();
        let always_true = inner.get_user_defined_typed_func("always_true");

        Self { inner, always_true }
    }

    fn always_true(&mut self) -> bool {
        let Self { always_true, inner } = self;
        let ctx = inner.write_context();

        always_true
            .call(inner.store_mut(), ctx)
            .expect("failed to call `always_true` function");

        let result = inner
            .store_mut()
            .data_mut()
            .0
            .take()
            .expect("always_true should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}

struct Nft {
    inner: TestCrate,
    mint: UserDefinedFn,
}

impl Nft {
    fn new() -> Self {
        let mut inner = Builder::new("bench-crate").build();
        let mint = inner.get_user_defined_typed_func("mint");

        Self { inner, mint }
    }

    fn mint(&mut self, id: u64) {
        let Self { mint, inner } = self;
        let ctx = inner.write_context();

        mint.call(inner.store_mut(), ctx)
            .expect("failed to call `mint` function");

        let result = inner
            .store_mut()
            .data_mut()
            .0
            .take()
            .expect("mint should always return something");

        borsh::from_slice(&result).expect("failed to deserialize result")
    }
}
