//! A client and types for the VM simulator. This crate allows for Rust
//! developers to construct tests for their programs completely in Rust.
//! Alternatively the [`Step`]s can be written in JSON and passed to the
//! Simulator binary directly.

use base64::{engine::general_purpose::STANDARD as b64, Engine};
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::{
    io::{BufRead, BufReader, Write},
    path::Path,
    process::{Child, Command, Stdio},
};
use thiserror::Error;
use wasmlanche_sdk::{
    borsh::{self, BorshDeserialize},
    Address, ExternalCallError,
};

mod id;
mod param;
mod simulator;

pub use context::TestContext;
pub use id::Id;
pub use param::Param;
pub use simulator::{build_simulator, Simulator, SimulatorResponseError};
