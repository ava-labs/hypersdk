//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
mod codec;
mod context;
mod id;
mod param;
mod simulator;
mod util;

pub use context::TestContext;
pub use id::Id;
pub use param::Param;
pub use simulator::{build_simulator, Simulator, SimulatorResponseError};
