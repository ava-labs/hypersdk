use wasmlanche_sdk::{public, state_keys, Context};

/// The program state keys.
#[state_keys]
pub enum StateKeys {
    State
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn write(context: Context<StateKeys>, value:i64) {
    let Context { program, .. } = context;
    program
    .state()
    .store(StateKeys::State, &value)
    .expect("failed to store state");
}


#[public]
pub fn read(context: Context<StateKeys>) -> i64 {
    let Context { program, .. } = context;
    program
    .state()
    .get(StateKeys::State)
    .expect("failed to get state")
}

#[public]
pub fn remove(context: Context<StateKeys>) -> Option<i64> {
    let Context { program, .. } = context;
    program
    .state()
    .delete(StateKeys::State)
    .expect("failed to get state")
}
