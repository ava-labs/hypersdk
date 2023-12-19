//! This module contains functionality for interacting with a `HyperSDK` `Program`
//! host. The host implements modules that can be imported into a Program
//! (guest).
mod crypto;
mod program;
mod state;
pub use crypto::{batch_verify_ed25519, verify_ed25519, SignedMessage};
pub(crate) use program::call as call_program;
#[allow(unused_imports)]
pub use state::*;
