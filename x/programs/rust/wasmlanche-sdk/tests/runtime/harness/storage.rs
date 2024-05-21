extern crate sdk_macros;
extern crate wasmlanche_sdk;

use self::sdk_macros::{public, state_keys};
use self::wasmlanche_sdk::Context;

#[state_keys]
enum StateKeys {
    Num,
}

#[public]
pub fn store_value(context: Context, value: usize) -> usize {
    match context.program.state().store(StateKeys::Num, &value) {
        Ok(_) => 1,
        Err(_) => 0,
    }
}

#[public]
pub fn get_value(context: Context) -> usize {
    context
        .program
        .state()
        .get(StateKeys::Num)
        .expect("failed to get value from storage")
}
