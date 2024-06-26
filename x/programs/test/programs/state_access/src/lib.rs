use wasmlanche_sdk::{public, state_keys, Context};

/// The program state keys.
#[state_keys]
pub enum StateKeys {
    State,
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn put(context: Context<StateKeys>, value: i64) {
    context
        .program()
        .state()
        .store(StateKeys::State, &value)
        .expect("failed to store state");
}

#[public]
pub fn get(context: Context<StateKeys>) -> Option<i64> {
    context
        .program()
        .state()
        .get::<i64>(StateKeys::State)
        .expect("failed to get state")
}

#[public]
pub fn delete(context: Context<StateKeys>) -> Option<i64> {
    context
        .program()
        .state()
        .delete(StateKeys::State)
        .expect("failed to get state")
}
