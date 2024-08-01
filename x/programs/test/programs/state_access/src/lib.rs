use wasmlanche_sdk::{public, state_schema, Context};

state_schema! {
    State => i64,
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn put(context: &mut Context, value: i64) {
    context
        .store_by_key(State, value)
        .expect("failed to store state");
}

#[public]
pub fn get(context: &mut Context) -> Option<i64> {
    context.get(State).expect("failed to get state")
}

#[public]
pub fn delete(context: &mut Context) -> Option<i64> {
    context.delete(State).expect("failed to get state")
}
