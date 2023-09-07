use wasmlanche_sdk::{public, store::State, types::Address};


#[public]
fn many_params(_: State, a : i64, b: i64, c : i64, d: i64, e: i64, f: i64) -> i64 {
    a + b + c + d + e + f
}

#[public]
fn while_true(_: State) -> bool {
    while true {
        let a = 1;
    }
    true
}
