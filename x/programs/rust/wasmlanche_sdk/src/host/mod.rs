//! This module contains functionality for interacting with a `HyperSDK` `Program`
//! host. The host implements modules that can be imported into a Program
//! (guest).
mod program;
mod state;
mod crypto;
pub(crate) use program::call as call_program;
pub use crypto::{verify_ed25519, batch_verify_ed25519};
pub use crypto::SignedMessage;
#[allow(unused_imports)]
pub use state::*;
