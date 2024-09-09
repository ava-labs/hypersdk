// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context};

#[public]
pub fn inc(ctx: &mut Context, external: Address, of: Address) {
    let ctx = ctx.to_extern(external, 1_000_000, 0);
    counter::inc(&ctx, of, 1);
}

#[public]
pub fn get_value(ctx: &mut Context, external: Address, of: Address) -> u64 {
    let ctx = ctx.to_extern(external, 1_000_000, 0);
    counter::get_value(&ctx, of)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn get_value_exists() {
        let mut ctx = Context::new();
        let external = Address::new([0; 33]);
        let of = Address::new([1; 33]);

        // mock `get_value` external contract call to return `value`
        let value = 5_u64;
        ctx.mock_function_call(external, "get_value", of, 1_000_000, 0, value);

        let value = get_value(&mut ctx, external, of);
        assert_eq!(value, 5);
    }
}
