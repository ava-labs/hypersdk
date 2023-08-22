use crate::errors::StorageError;
use crate::host::init_program_storage;
use crate::store::ProgramContext;
use serde::Serialize;
use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum ProgramError {
    #[error("{0}")]
    Store(#[from] StorageError),

    #[error("Program Context Uninitialized")]
    UninitalizedContextError(),
}

/// Program represents a program and its associated fields.
pub struct Program {
    ctx: ProgramContext,
}

impl Program {
    pub fn new() -> Self {
        // get the program_id from the host
        Program {
            ctx: init_program_storage(),
        }
    }
    pub fn add_field<T>(&mut self, name: String, value: T) -> Result<(), ProgramError>
    where
        T: Serialize,
    {
        Ok(self.ctx.store_value(&name, &value)?)
    }
}

impl From<Program> for i64 {
    fn from(p: Program) -> Self {
        p.ctx.program_id
    }
}
