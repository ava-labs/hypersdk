// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use wasmlanche::{public, Address, Context};

#[public]
pub fn inc(context: &mut Context, external: Address, of: Address) {
    let ctx = context.new_external_call_context(external, 1_000_000, 0);
    counter::inc(&ctx, of, 1);
}

#[public]
pub fn get_value(context: &mut Context, external: Address, of: Address) -> u64 {
    let ctx = context.new_external_call_context(external, 1_000_000, 0);
    counter::get_value(&ctx, of)
}

#[cfg(test)]
mod tests {
    use crate::*;
    #[test]
    fn inc_and_get_value() {
        let owner = Address::new([1; 33]);
        let counter = Address::new([2; 33]);
        let mut context = Context::new_test_context();

        inc(&mut context, counter, owner);

        let response = get_value(&mut context, counter, owner);

        assert_eq!(response, 1);
    }
}
