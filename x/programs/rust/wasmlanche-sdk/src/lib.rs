#![deny(clippy::pedantic)]

/// State-related operations in programs.
pub mod state;

/// Program types.
pub mod types;

mod logging;
mod memory;
mod program;

pub use self::{
    logging::{log, register_panic},
    memory::HostPtr,
    program::{ExternalCallError, Program},
};
use crate::types::{Gas, Id};

#[cfg(feature = "build")]
pub mod build;

use borsh::{BorshDeserialize, BorshSerialize};
pub use sdk_macros::{public, state_keys};
use types::Address;

/// Representation of the context that is passed to programs at runtime.
#[cfg_attr(feature = "debug", derive(Debug))]
pub struct Context<K = ()> {
    pub program: Program<K>,
    pub actor: Address,
    pub height: u64,
    pub timestamp: u64,
    pub action_id: Id,
}

impl<K> BorshSerialize for Context<K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            program,
            actor,
            height,
            timestamp,
            action_id,
        } = self;
        BorshSerialize::serialize(program, writer)?;
        BorshSerialize::serialize(actor, writer)?;
        BorshSerialize::serialize(height, writer)?;
        BorshSerialize::serialize(timestamp, writer)?;
        BorshSerialize::serialize(action_id, writer)?;
        Ok(())
    }
}

impl<K> BorshDeserialize for Context<K> {
    fn deserialize_reader<R: std::io::prelude::Read>(reader: &mut R) -> std::io::Result<Self> {
        let program = BorshDeserialize::deserialize_reader(reader)?;
        let actor = BorshDeserialize::deserialize_reader(reader)?;
        let height = BorshDeserialize::deserialize_reader(reader)?;
        let timestamp = BorshDeserialize::deserialize_reader(reader)?;
        let action_id = BorshDeserialize::deserialize_reader(reader)?;
        Ok(Self {
            program,
            actor,
            height,
            timestamp,
            action_id,
        })
    }
}

/// Special context that is passed to external programs.
pub struct ExternalCallContext {
    program: Program,
    max_units: Gas,
}

impl ExternalCallContext {
    pub fn new(program: Program, max_units: Gas) -> Self {
        Self { program, max_units }
    }

    pub fn program(&self) -> &Program {
        &self.program
    }

    pub fn max_units(&self) -> Gas {
        self.max_units
    }
}
