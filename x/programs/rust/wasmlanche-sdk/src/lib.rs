// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#![deny(clippy::pedantic)]

#[cfg(feature = "build")]
pub mod build;
/// State-related operations in programs.
pub mod state;

mod context;
mod logging;
mod memory;
mod program;
mod types;

pub use self::{
    context::{Context, ExternalCallContext},
    program::{send, ExternalCallError, Program},
    types::{Address, Gas, Id, ID_LEN},
};
#[doc(hidden)]
pub use self::{
    logging::{log, register_panic},
    memory::HostPtr,
};

pub use sdk_macros::{public, state_schema};

// re-exports
pub use borsh;
pub use bytemuck;
