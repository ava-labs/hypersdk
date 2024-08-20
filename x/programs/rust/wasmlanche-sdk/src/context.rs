// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{
    state::{self, Error, IntoPairs, Schema, State},
    types::{Address, GasUnits},
    HostPtr, Id, Program,
};
use borsh::BorshDeserialize;
use std::{cell::RefCell, collections::HashMap};

pub type CacheKey = Box<[u8]>;
pub type CacheValue = Vec<u8>;

/// Representation of the context that is passed to programs at runtime.
#[cfg_attr(feature = "debug", derive(Debug))]
pub struct Context {
    program: Program,
    actor: Address,
    height: u64,
    timestamp: u64,
    action_id: Id,
    state_cache: RefCell<HashMap<CacheKey, Option<CacheValue>>>,
}

impl Drop for Context {
    fn drop(&mut self) {
        State::new(&self.state_cache).flush();
    }
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

    /// See [`State::get`].
    /// # Errors
    /// See [`State::get`].
    pub fn get<Key>(&mut self, key: Key) -> Result<Option<Key::Value>, state::Error>
    where
        Key: Schema,
    {
        key.get(self)
    }

    /// Should not use this function directly.
    /// # Errors
    /// Errors when there's an issue deserializing.
    pub fn get_with_raw_key<V>(&mut self, key: &[u8]) -> Result<Option<V>, state::Error>
    where
        V: BorshDeserialize,
    {
        State::new(&self.state_cache).get_with_raw_key(key)
    }

    /// See [`State::store_by_key`].
    /// # Errors
    /// See [`State::store_by_key`].
    pub fn store_by_key<K>(&self, key: K, value: K::Value) -> Result<(), Error>
    where
        K: Schema,
    {
        State::new(&self.state_cache).store_by_key(key, value)
    }

    /// See [`State::store`].
    /// # Errors
    /// See [`State::store`].    
    pub fn store<Pairs: IntoPairs>(&self, pairs: Pairs) -> Result<(), Error> {
        State::new(&self.state_cache).store(pairs)
    }

    /// See [`State::delete`].
    /// # Errors
    /// See [`State::delete`].
    pub fn delete<K: Schema>(&self, key: K) -> Result<Option<K::Value>, Error> {
        State::new(&self.state_cache).delete(key)
    }

    /// Deploy an instance of the specified program and returns the account of the new instance
    /// # Panics
    /// Panics if there was an issue deserializing the account
    #[must_use]
    pub fn deploy(&self, program_id: Id, account_creation_data: &[u8]) -> Program {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "deploy"]
            fn deploy(ptr: *const u8, len: usize) -> HostPtr;
        }
        let ptr =
            borsh::to_vec(&(program_id, account_creation_data)).expect("failed to serialize args");

        let bytes = unsafe { deploy(ptr.as_ptr(), ptr.len()) };

        borsh::from_slice(&bytes).expect("failed to deserialize the account")
    }
}

/// Special context that is passed to external programs.
#[allow(clippy::module_name_repetitions)]
pub struct ExternalCallContext {
    program: Program,
    max_units: GasUnits,
    value: u64,
}

impl ExternalCallContext {
    #[must_use]
    pub fn new(program: Program, max_units: GasUnits, value: u64) -> Self {
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
    pub fn max_units(&self) -> &GasUnits {
        &self.max_units
    }

    #[must_use]
    pub fn value(&self) -> u64 {
        self.value
    }
}
