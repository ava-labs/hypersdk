//! This module contains functionality for interacting with a `HyperSDK` `Program`
//! host. The host implements modules that can be imported into a Program
//! (guest).
mod program;
mod state;

pub(crate) use program::call as call_program;
#[allow(unused_imports)]
pub use state::*;
