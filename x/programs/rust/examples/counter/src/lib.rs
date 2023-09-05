use expose_macro::expose;
use wasmlanche_sdk::store::State;
use wasmlanche_sdk::types::Address;

/// Initializes the program. This program maps addresses with a count.
#[expose]
fn init(state: State) -> bool {
    state.store_value("counter", &0_i64).is_ok()
}

/// Increments the count at the address by the amount.
#[expose]
fn inc(state: State, to: Address, amount: i64) -> bool {
    let counter = amount + value(state, to);
    // dont check for error/ok
    state.store_map_value("counts", &to, &counter).is_ok()
}

/// Gets the count at the address.
#[expose]
fn value(state: State, of: Address) -> i64 {
    state.get_map_value("counts", &of).unwrap_or(0)
}
