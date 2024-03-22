#![deny(clippy::pedantic)]

pub mod params;
pub mod state;
pub mod types;

mod memory;
mod program;

pub use self::{
    memory::from_host_ptr,
    params::{serialize_param, Params},
    program::Program,
};

#[cfg(feature = "build")]
pub mod build;

pub use sdk_macros::{public, state_keys};

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("State error: {0}")]
    State(#[from] state::Error),
    #[error("Param error: {0}")]
    Param(#[from] std::io::Error),
}
