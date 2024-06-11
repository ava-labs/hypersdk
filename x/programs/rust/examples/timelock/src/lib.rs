#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Gas, Program};

#[state_keys]
pub enum StateKeys {
    /// The count of this program. Key prefix 0x0 + address
    Counter(Address),
}

const LOCK_TIME: u64 = 3600;

#[public]
pub fn start(context: Context<StateKeys>, to: Address, data: Vec<u8>) {
    todo!()
}

#[public]
pub fn execute(context: Context<StateKeys>) {
    todo!()
}
