#![deny(clippy::pedantic)]

pub mod state;
pub mod types;

mod logging;
mod memory;
mod program;

pub use self::{
    logging::log,
    memory::HostPtr,
    program::{Program, PROGRAM_ID_LEN},
};

#[cfg(feature = "build")]
pub mod build;

use borsh::{BorshDeserialize, BorshSerialize};
pub use sdk_macros::{public, state_keys};
use types::Address;

pub type Gas = i64;

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("State error: {0}")]
    State(#[from] state::Error),
    #[error("Param error: {0}")]
    Param(#[from] std::io::Error),
}

#[derive(Clone, Debug)]
pub struct Context<K = ()> {
    pub program: Program<K>,
    pub account: Address,
    pub actor: Address,
}

impl<K> BorshSerialize for Context<K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            program,
            account,
            actor,
        } = self;
        BorshSerialize::serialize(program, writer)?;
        BorshSerialize::serialize(account, writer)?;
        BorshSerialize::serialize(actor, writer)?;
        Ok(())
    }
}

impl<K> BorshDeserialize for Context<K> {
    fn deserialize_reader<R: std::io::prelude::Read>(reader: &mut R) -> std::io::Result<Self> {
        let program: Program<K> = BorshDeserialize::deserialize_reader(reader)?;
        let account: Address = BorshDeserialize::deserialize_reader(reader)?;
        let actor: Address = BorshDeserialize::deserialize_reader(reader)?;
        Ok(Self {
            program,
            account,
            actor,
        })
    }
}
