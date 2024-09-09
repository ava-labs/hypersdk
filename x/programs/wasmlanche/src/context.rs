// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

extern crate alloc;

use crate::{
    host::Accessor,
    state::{Cache, Error, IntoPairs, Schema},
    types::{Address, ProgramId},
    Gas, Id,
};
use alloc::{boxed::Box, vec::Vec};
use borsh::{BorshDeserialize, BorshSerialize};
use displaydoc::Display;

pub type CacheKey = Box<[u8]>;
pub type CacheValue = Vec<u8>;

/// Representation of the context that is passed to programs at runtime.
#[cfg_attr(feature = "debug", derive(Debug))]
pub enum Context {
    /// The context that is passed to the program when it is called by the host.
    Injected(Injected),
    #[cfg(feature = "bindings")]
    /// The context that is passed to the program when it is called by another program.
    External(ExternalCallContext),
}

#[doc(hidden)]
pub struct Injected {
    contract_address: Address,
    actor: Address,
    height: u64,
    timestamp: u64,
    action_id: Id,
    state_cache: Cache,
    host_accessor: Accessor,
}

#[cfg(feature = "debug")]
mod debug {
    use super::Injected;
    use core::fmt::{Debug, Formatter, Result};

    macro_rules! debug_struct_fields {
        ($f:expr, $struct_name:ty, $($name:expr),* $(,)*) => {
            $f.debug_struct(stringify!(struct_name))
                $(.field(stringify!($name), $name))*
                .finish()
        };
    }

    impl Debug for Injected {
        fn fmt(&self, f: &mut Formatter<'_>) -> Result {
            let Self {
                contract_address,
                actor,
                height,
                timestamp,
                action_id,
                state_cache: _,
                host_accessor: _,
            } = self;

            debug_struct_fields!(
                f,
                Context,
                contract_address,
                actor,
                height,
                timestamp,
                action_id
            )
        }
    }
}

impl BorshDeserialize for Context {
    fn deserialize_reader<R: borsh::io::Read>(reader: &mut R) -> borsh::io::Result<Self> {
        let contract_address = Address::deserialize_reader(reader)?;
        let actor = Address::deserialize_reader(reader)?;
        let height = u64::deserialize_reader(reader)?;
        let timestamp = u64::deserialize_reader(reader)?;
        let action_id = Id::deserialize_reader(reader)?;

        let ctx = Injected {
            contract_address,
            actor,
            height,
            timestamp,
            action_id,
            state_cache: Cache::new(),
            host_accessor: Accessor::new(),
        };

        Ok(Self::Injected(ctx))
    }
}

impl Context {
    #[cfg(feature = "test")]
    #[must_use]
    pub fn new() -> Self {
        Self::Injected(Injected {
            contract_address: Address::default(),
            actor: Address::default(),
            height: 0,
            timestamp: 0,
            action_id: Id::default(),
            state_cache: Cache::new(),
            host_accessor: Accessor::new(),
        })
    }

    #[must_use]
    pub fn contract_address(&self) -> Address {
        match self {
            Context::Injected(ctx) => ctx.contract_address,
            #[cfg(feature = "bindings")]
            Context::External(ctx) => ctx.contract_address(),
        }
    }

    /// Returns the address of the actor that is executing the program.
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn actor(&self) -> Address {
        match self {
            Context::Injected(ctx) => ctx.actor,
            #[cfg(feature = "bindings")]
            Context::External(_) => panic!("not supported"),
        }
    }

    #[cfg(feature = "test")]
    /// Sets the actor of the context
    /// # Panics
    /// Panics if the context was not injected
    pub fn set_actor(&mut self, actor: Address) {
        match self {
            Context::Injected(ctx) => ctx.actor = actor,
            #[cfg(feature = "bindings")]
            Context::External(_) => panic!("not supported"),
        }
    }

    /// Returns the block-height
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn height(&self) -> u64 {
        match self {
            Context::Injected(ctx) => ctx.height,
            #[cfg(feature = "bindings")]
            Context::External(_) => panic!("not supported"),
        }
    }

    /// Returns the block-timestamp
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn timestamp(&self) -> u64 {
        match self {
            Context::Injected(ctx) => ctx.timestamp,
            #[cfg(feature = "bindings")]
            Context::External(_) => panic!("not supported"),
        }
    }

    /// Returns the action-id
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn action_id(&self) -> Id {
        match self {
            Context::Injected(ctx) => ctx.action_id,
            #[cfg(feature = "bindings")]
            Context::External(_) => panic!("not supported"),
        }
    }

    /// # Panics
    /// Panics if the context was not injected
    fn state_cache(&mut self) -> &mut Cache {
        match self {
            Context::Injected(ctx) => &mut ctx.state_cache,
            #[cfg(feature = "bindings")]
            Context::External(_) => panic!("not supported"),
        }
    }

    pub(crate) fn host_accessor(&self) -> &Accessor {
        match self {
            Context::Injected(ctx) => &ctx.host_accessor,
            #[cfg(feature = "bindings")]
            Context::External(ctx) => &ctx.host_accessor,
        }
    }

    /// Get a value from state.
    ///
    /// # Errors
    /// Returns an [`Error`] if the key cannot be serialized or if
    /// the host fails to read the key and value.
    ///
    /// # Panics
    /// Panics if the value cannot be converted from i32 to usize.
    pub fn get<Key>(&mut self, key: Key) -> Result<Option<Key::Value>, Error>
    where
        Key: Schema,
    {
        key.get(self.state_cache())
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store_by_key<K>(&mut self, key: K, value: K::Value) -> Result<(), Error>
    where
        K: Schema,
    {
        self.state_cache().store_by_key(key, value)
    }

    /// Store a list of tuple of key and value to the host storage.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    pub fn store<Pairs: IntoPairs>(&mut self, pairs: Pairs) -> Result<(), Error> {
        self.state_cache().store(pairs)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the value is inexistent
    /// or if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    pub fn delete<K: Schema>(&mut self, key: K) -> Result<Option<K::Value>, Error> {
        self.state_cache().delete(key)
    }

    /// Deploy an instance of the specified program and returns the account of the new instance
    /// # Panics
    /// Panics if there was an issue deserializing the account
    #[must_use]
    pub fn deploy(&mut self, program_id: ProgramId, account_creation_data: &[u8]) -> Address {
        let ptr =
            borsh::to_vec(&(program_id, account_creation_data)).expect("failed to serialize args");

        let bytes = self.host_accessor().deploy(&ptr);

        borsh::from_slice(&bytes).expect("failed to deserialize the account")
    }

    /// Gets the remaining fuel available to this program
    /// # Panics
    /// Panics if there was an issue deserializing the remaining fuel
    #[must_use]
    pub fn remaining_fuel(&self) -> u64 {
        let bytes = self.host_accessor().get_remaining_fuel();

        borsh::from_slice::<u64>(&bytes).expect("failed to deserialize the remaining fuel")
    }

    /// Gets the balance for the specified address
    /// # Panics
    /// Panics if there was an issue deserializing the balance
    #[must_use]
    pub fn get_balance(&mut self, account: Address) -> u64 {
        let ptr = borsh::to_vec(&account).expect("failed to serialize args");
        let bytes = self.host_accessor().get_balance(&ptr);

        borsh::from_slice(&bytes).expect("failed to deserialize the balance")
    }

    /// Transfer currency from the calling program to the passed address
    /// # Panics
    /// Panics if there was an issue deserializing the result
    /// # Errors
    /// Errors if there are insufficient funds
    pub fn send(&self, to: Address, amount: u64) -> Result<(), ExternalCallError> {
        let ptr = borsh::to_vec(&(to, amount)).expect("failed to serialize args");

        let bytes = self.host_accessor().send_value(&ptr);

        borsh::from_slice(&bytes).expect("failed to deserialize the result")
    }

    /// Attempts to call a function `name` with `args` on the given program. This method
    /// is used to call functions on external programs.
    /// # Errors
    /// Returns a [`ExternalCallError`] if the call fails.
    /// # Panics
    /// Will panic if the args cannot be serialized
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_program<T: BorshDeserialize>(
        &mut self,
        address: Address,
        function_name: &str,
        args: &[u8],
        max_units: Gas,
        max_value: u64,
    ) -> Result<T, ExternalCallError> {
        call_function(
            self.host_accessor(),
            address,
            function_name,
            args,
            max_units,
            max_value,
        )
    }

    /// Attempts to call a function `name` with `args` on the given program. This method
    /// is used to call functions on external programs.
    /// # Errors
    /// Returns a [`ExternalCallError`] if the call fails.
    /// # Panics
    /// Will panic if the args cannot be serialized
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_function<T: BorshDeserialize>(
        &self,
        function_name: &str,
        args: &[u8],
    ) -> Result<T, ExternalCallError> {
        match self {
            Context::Injected(_) => call_function(
                self.host_accessor(),
                self.contract_address(),
                function_name,
                args,
                self.remaining_fuel(),
                0,
            ),
            #[cfg(feature = "bindings")]
            Context::External(ctx) => ctx.call_function(function_name, args),
        }
    }

    #[cfg(feature = "bindings")]
    #[must_use]
    pub fn to_extern(&self, address: Address, max_units: Gas, value: u64) -> Self {
        Self::External(ExternalCallContext::new(
            self.host_accessor().clone(),
            address,
            max_units,
            value,
        ))
    }
}

/// An error that is returned from call to public functions.
#[derive(Debug, Display, BorshSerialize, BorshDeserialize, PartialEq, Eq)]
#[repr(u8)]
#[non_exhaustive]
#[borsh(use_discriminant = true)]
pub enum ExternalCallError {
    /// an error happened during execution
    ExecutionFailure = 0,
    /// the call panicked
    CallPanicked = 1,
    /// not enough fuel to cover the execution
    OutOfFuel = 2,
    /// insufficient funds
    InsufficientFunds = 3,
}

/// Special context that is passed to external programs.
#[allow(clippy::module_name_repetitions)]
#[cfg_attr(feature = "debug", derive(Debug))]
#[cfg(feature = "bindings")]
pub struct ExternalCallContext {
    contract_address: Address,
    max_units: Gas,
    value: u64,
    host_accessor: Accessor,
}

#[cfg(feature = "bindings")]
impl ExternalCallContext {
    #[must_use]
    fn new(host_accessor: Accessor, contract_address: Address, max_units: Gas, value: u64) -> Self {
        Self {
            contract_address,
            max_units,
            value,
            host_accessor,
        }
    }

    /// Attempts to call a function `name` with `args` on the given program. This method
    /// is used to call functions on external programs.
    /// # Errors
    /// Returns a [`ExternalCallError`] if the call fails.
    /// # Panics
    /// Will panic if the args cannot be serialized
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_function<T: BorshDeserialize>(
        &self,
        function_name: &str,
        args: &[u8],
    ) -> Result<T, ExternalCallError> {
        call_function(
            &self.host_accessor,
            self.contract_address,
            function_name,
            args,
            self.max_units(),
            self.value(),
        )
    }

    #[must_use]
    pub fn contract_address(&self) -> Address {
        self.contract_address
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

#[cfg(feature = "test")]
impl Default for Context {
    fn default() -> Self {
        Self::new()
    }
}

fn call_function<T: BorshDeserialize>(
    host_accessor: &Accessor,
    address: Address,
    function_name: &str,
    args: &[u8],
    max_units: Gas,
    max_value: u64,
) -> Result<T, ExternalCallError> {
    let args = CallContractArgs::new(&address, function_name, args, max_units, max_value);

    let args_bytes = borsh::to_vec(&args).expect("failed to serialize args");

    let bytes = host_accessor.call_program(&args_bytes);

    borsh::from_slice(&bytes).expect("failed to deserialize")
}

#[derive(BorshSerialize)]
pub struct CallContractArgs<'a> {
    target: &'a Address,
    function: &'a [u8],
    args: &'a [u8],
    max_units: Gas,
    max_value: u64,
}

impl<'a> CallContractArgs<'a> {
    pub fn new(
        target: &'a Address,
        function: &'a str,
        args: &'a [u8],
        max_units: Gas,
        max_value: u64,
    ) -> Self {
        Self {
            target,
            function: function.as_bytes(),
            args,
            max_units,
            max_value,
        }
    }
}
