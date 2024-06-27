use wasmlanche_sdk::{public, Context};

mod state_keys {
    use borsh::BorshDeserialize;
    use borsh::BorshSerialize;
    use wasmlanche_sdk::state::{Key, StateSchema};

    #[derive(Copy, Clone, PartialEq, Eq, Hash)]
    pub struct State;

    unsafe impl Key for State {}

    impl StateSchema for State {
        type SchemaType = i64;
    }

    impl BorshSerialize for State {
        fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
            todo!()
        }
    }
}

/// Initializes the program with a name, symbol, and total supply.
#[public]
pub fn put(context: Context<state_keys::StateKeys>, value: i64) {
    context
        .program()
        .state()
        .store(state_keys::State, &value)
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

// #[public]
// pub fn delete(context: Context<StateKeys>) -> Option<i64> {
//     context
//         .program()
//         .state()
//         .delete(StateKeys::State)
//         .expect("failed to get state")
// }
