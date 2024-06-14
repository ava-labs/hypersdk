#![deny(clippy::pedantic)]

pub mod state;
pub mod types;

mod logging;
mod memory;
mod program;

pub use self::{
    logging::{log, register_panic},
    memory::HostPtr,
    program::{ExternalCallError, Program},
};

#[cfg(feature = "build")]
pub mod build;

use borsh::{BorshDeserialize, BorshSerialize};
pub use sdk_macros::{public, state_keys};
use types::Address;

pub const ID_LEN: usize = 32;
pub type Id = [u8; ID_LEN];
pub type Gas = u64;

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("State error: {0}")]
    State(#[from] state::Error),
    #[error("Param error: {0}")]
    Param(#[from] std::io::Error),
}

#[cfg_attr(feature = "debug", derive(Debug))]
pub struct Context<K = ()> {
    pub program: Program<K>,
    pub actor: Address,
    pub height: u64,
    pub action_id: Id,
}

impl<K> BorshSerialize for Context<K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            program,
            actor,
            height,
            action_id,
        } = self;
        BorshSerialize::serialize(program, writer)?;
        BorshSerialize::serialize(actor, writer)?;
        BorshSerialize::serialize(height, writer)?;
        BorshSerialize::serialize(action_id, writer)?;
        Ok(())
    }
}

impl<K> BorshDeserialize for Context<K> {
    fn deserialize_reader<R: std::io::prelude::Read>(reader: &mut R) -> std::io::Result<Self> {
        let program: Program<K> = BorshDeserialize::deserialize_reader(reader)?;
        let actor: Address = BorshDeserialize::deserialize_reader(reader)?;
        let height: u64 = BorshDeserialize::deserialize_reader(reader)?;
        let action_id: Id = BorshDeserialize::deserialize_reader(reader)?;
        Ok(Self {
            program,
            actor,
            height,
            action_id,
        })
    }
}

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
