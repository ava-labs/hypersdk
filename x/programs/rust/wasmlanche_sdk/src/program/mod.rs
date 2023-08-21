use crate::errors::StorageError;
use crate::host::init_program_storage;
use crate::store::{ProgramContext, Store, Tag};
use crate::types::Address;
use serde::Serialize;
use serde_json::{from_slice, to_vec};

use std::borrow::Cow;
use std::collections::HashMap;
use thiserror::Error;

#[derive(Clone, Error, Debug)]
pub enum ProgramError {
    #[error("{0}")]
    Store(#[from] StorageError),
}

/// Program represents a program and its associated fields.
pub struct Program<T>
where
    T: Serialize,
{
    fields: HashMap<String, T>,
}

impl<T> Program<T>
where
    T: Serialize,
{
    pub fn new() -> Self {
        Program {
            fields: HashMap::new(),
        }
    }
    pub fn add_field(&mut self, name: String, val: T)
    where
        T: Serialize,
    {
        self.fields.insert(name, val);
    }
    /// Initializes all the fields in the program and stores them in the host.
    pub fn publish(self) -> Result<ProgramContext, ProgramError> {
        // get the program_id from the host
        let ctx: ProgramContext = init_program_storage();
        // iterate through fields an set them in the host
        for (key, value) in &self.fields {
            ctx.store_value(key, value)?;
        }
        Ok(ctx)
    }
}
