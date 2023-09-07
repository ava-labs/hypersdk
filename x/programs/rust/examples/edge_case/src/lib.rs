use expose_macro::expose;
use wasmlanche_sdk::store::Context;

#[expose]
fn init(_: Context) -> bool {
    // Initialize the program with no fields
    true
}

#[expose]
fn many_params(_: Context, a : i64, b: i64, c : i64, d: i64, e: i64, f: i64) -> i64 {
    a + b + c + d + e + f
}

#[expose]
fn while_true(_: Context) -> bool {
    while true {
        let a = 1;
    }
    true
}
