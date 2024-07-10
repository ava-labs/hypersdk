use crate::{
    state::{Error, State},
    types::Address,
    Gas, Id, Program,
};
use borsh::{BorshDeserialize, BorshSerialize};
use bytemuck::NoUninit;
use std::{cell::RefCell, collections::HashMap};

/// Representation of the context that is passed to programs at runtime.
#[cfg_attr(feature = "debug", derive(Debug))]
pub struct Context {
    program: Program,
    actor: Address,
    height: u64,
    timestamp: u64,
    action_id: Id,
    state_cache: RefCell<HashMap<Vec<u8>, Option<Vec<u8>>>>,
}

impl BorshDeserialize for Context {
    fn deserialize_reader<R: std::io::Read>(reader: &mut R) -> std::io::Result<Self> {
        let program = Program::deserialize_reader(reader)?;
        let actor = Address::deserialize_reader(reader)?;
        let height = u64::deserialize_reader(reader)?;
        let timestamp = u64::deserialize_reader(reader)?;
        let action_id = Id::deserialize_reader(reader)?;

        Ok(Self {
            program,
            actor,
            height,
            timestamp,
            action_id,
            state_cache: RefCell::new(HashMap::new()),
        })
    }
}

impl Context {
    pub fn program(&self) -> &Program {
        &self.program
    }

    pub fn actor(&self) -> Address {
        self.actor
    }

    pub fn height(&self) -> u64 {
        self.height
    }

    pub fn timestamp(&self) -> u64 {
        self.timestamp
    }

    pub fn action_id(&self) -> Id {
        self.action_id
    }
}

impl Context {
    /// See [`State::get`].
    /// # Errors
    /// See [`State::get`].
    pub fn get<K, V>(&self, key: &K) -> Result<Option<V>, crate::state::Error>
    where
        K: NoUninit,
        V: BorshDeserialize,
    {
        State::new(&self.state_cache).get(key)
    }

    /// See [`State::store_by_key`].
    /// # Errors
    /// See [`State::store_by_key`].
    pub fn store_by_key<K, V>(&self, key: &K, value: &V) -> Result<(), crate::state::Error>
    where
        K: NoUninit,
        V: BorshSerialize,
    {
        State::new(&self.state_cache).store_by_key(key, value)
    }

    /// See [`State::store`].
    /// # Errors
    /// See [`State::store`].    
    pub fn store<'b, V: BorshSerialize + 'b, Pairs: IntoIterator<Item = (Vec<u8>, &'b V)>>(
        &self,
        pairs: Pairs,
    ) -> Result<(), Error> {
        State::new(&self.state_cache).store(pairs)
    }

    /// See [`State::delete`].
    /// # Errors
    /// See [`State::delete`].
    pub fn delete<K: NoUninit, V: BorshDeserialize>(&self, key: &K) -> Result<Option<V>, Error> {
        State::new(&self.state_cache).delete(key)
    }
}

/// Special context that is passed to external programs.
#[allow(clippy::module_name_repetitions)]
pub struct ExternalCallContext {
    program: Program,
    max_units: Gas,
    value: u64,
}

impl ExternalCallContext {
    #[must_use]
    pub fn new(program: Program, max_units: Gas, value: u64) -> Self {
        Self {
            program,
            max_units,
            value,
        }
    }

    #[must_use]
    pub fn program(&self) -> &Program {
        &self.program
    }

    #[must_use]
    pub fn max_units(&self) -> Gas {
        self.max_units
    }

    #[must_use]
    pub fn value(&self) -> u64 {
        self.value
    }
}
