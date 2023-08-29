use expose_macro::expose;
use wasmlanche_sdk::store::Context;
use wasmlanche_sdk::types::Address;

/// Initializes the program. This program maps addresses with a count.
#[expose]
fn init(ctx: Context) -> bool {
    ctx.store_value("counter", &0_i64).is_ok()
}

/// Increments the count at the address by the amount.
#[expose]
fn inc(ctx: Context, to: Address, amount: i64) -> bool {
    let counter = amount + value(ctx, to);
    // dont check for error/ok
    ctx.store_map_value("counts", &to, &counter).is_ok()
}

/// Gets the count at the address.
#[expose]
fn value(ctx: Context, of: Address) -> i64 {
    ctx.get_map_value("counts", &of).unwrap_or(0)
}
