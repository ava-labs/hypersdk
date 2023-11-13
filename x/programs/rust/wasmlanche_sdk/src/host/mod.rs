//! This module contains functionality for interacting with a `HyperSDK` `Program`
//! host. The host implements modules that can be imported into a Program
//! (guest).
mod program;
mod state;
mod crypto;
pub(crate) use program::call as call_program;
pub use crypto::verify_ed25519;
#[allow(unused_imports)]
pub use state::*;
