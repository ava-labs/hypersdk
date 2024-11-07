// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

extern crate alloc;

use crate::{
    host::{Accessor, CallContractArgs},
    state::{Cache, Error, IntoPairs, Schema},
    types::{Address, ContractId},
    Gas, Id,
};
use alloc::{boxed::Box, vec::Vec};
use borsh::{BorshDeserialize, BorshSerialize};
use displaydoc::Display;

pub type CacheKey = Box<[u8]>;
pub type CacheValue = Vec<u8>;

#[doc(hidden)]
pub struct Context {
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
    use super::Context;
    use core::fmt::{Debug, Formatter, Result};

    macro_rules! debug_struct_fields {
        ($f:expr, $struct_name:ty, $($name:expr),* $(,)*) => {
            $f.debug_struct(stringify!(struct_name))
                $(.field(stringify!($name), $name))*
                .finish()
        };
    }

    impl Debug for Context {
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

        let ctx = Context {
            contract_address,
            actor,
            height,
            timestamp,
            action_id,
            state_cache: Cache::new(),
            host_accessor: Accessor::new(),
        };

        Ok(ctx)
    }
}

impl Context {
    #[must_use]
    pub fn contract_address(&self) -> Address {
        self.contract_address
    }

    /// Returns the address of the actor that is executing the contract.
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn actor(&self) -> Address {
        self.actor
    }

    #[cfg(feature = "test")]
    /// Sets the actor of the context
    /// # Panics
    /// Panics if the context was not injected
    pub fn set_actor(&mut self, actor: Address) {
        self.actor = actor;
    }

    /// Returns the block-height
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn height(&self) -> u64 {
        self.height
    }

    /// Returns the block-timestamp
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn timestamp(&self) -> u64 {
        self.timestamp
    }

    /// Returns the action-id
    /// # Panics
    /// Panics if the context was not injected
    #[must_use]
    pub fn action_id(&self) -> Id {
        self.action_id
    }

    /// Get a value from state.
    ///
    /// # Errors
    /// Returns an [`Error`] if the key cannot be serialized or if
    /// the host fails to read the key and value.
    ///
    /// # Panics
    /// Panics if the value cannot be converted from i32 to usize.
    #[inline]
    pub fn get<Key>(&mut self, key: Key) -> Result<Option<Key::Value>, Error>
    where
        Key: Schema,
    {
        key.get(&mut self.state_cache)
    }

    /// Store a key and value to the host storage. If the key already exists,
    /// the value will be overwritten.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    #[inline]
    pub fn store_by_key<K>(&mut self, key: K, value: K::Value) -> Result<(), Error>
    where
        K: Schema,
    {
        self.state_cache.store_by_key(key, value)
    }

    /// Store a list of tuple of key and value to the host storage.
    /// # Errors
    /// Returns an [`Error`] if the key or value cannot be
    /// serialized or if the host fails to handle the operation.
    #[inline]
    pub fn store<Pairs: IntoPairs>(&mut self, pairs: Pairs) -> Result<(), Error> {
        self.state_cache.store(pairs)
    }

    /// Delete a value from the hosts's storage.
    /// # Errors
    /// Returns an [Error] if the value is inexistent
    /// or if the key cannot be serialized
    /// or if the host fails to delete the key and the associated value
    #[inline]
    pub fn delete<K: Schema>(&mut self, key: K) -> Result<Option<K::Value>, Error> {
        self.state_cache.delete(key)
    }

    /// Deploy an instance of the specified contract and returns the account of the new instance
    /// # Panics
    /// Panics if there was an issue deserializing the account
    #[must_use]
    #[inline]
    pub fn deploy(&mut self, contract_id: ContractId, account_creation_data: &[u8]) -> Address {
        let ptr =
            borsh::to_vec(&(contract_id, account_creation_data)).expect("failed to serialize args");
        let bytes = self.host_accessor.deploy(&ptr);

        borsh::from_slice(&bytes).expect("failed to deserialize the account")
    }

    /// Gets the remaining fuel available to this contract
    /// # Panics
    /// Panics if there was an issue deserializing the remaining fuel
    #[must_use]
    #[inline]
    pub fn remaining_fuel(&self) -> u64 {
        let bytes = self.host_accessor.get_remaining_fuel();

        borsh::from_slice::<u64>(&bytes).expect("failed to deserialize the remaining fuel")
    }

    /// Gets the balance for the specified address
    /// # Panics
    /// Panics if there was an issue deserializing the balance
    #[must_use]
    #[inline]
    pub fn get_balance(&mut self, account: Address) -> u64 {
        let ptr = borsh::to_vec(&account).expect("failed to serialize args");
        let bytes = self.host_accessor.get_balance(&ptr);

        borsh::from_slice(&bytes).expect("failed to deserialize the balance")
    }

    /// Transfer currency from the calling contract to the passed address
    /// # Panics
    /// Panics if there was an issue deserializing the result
    /// # Errors
    /// Errors if there are insufficient funds
    #[inline]
    pub fn send(&self, to: Address, amount: u64) -> Result<(), ExternalCallError> {
        let ptr = borsh::to_vec(&(to, amount)).expect("failed to serialize args");
        let bytes = self.host_accessor.send_value(&ptr);

        borsh::from_slice(&bytes).expect("failed to deserialize the result")
    }

    /// Attempts to call a function `name` with `args` on the given contract. This method
    /// is used to call functions on external contracts.
    /// # Errors
    /// Returns a [`ExternalCallError`] if the call fails.
    /// # Panics
    /// Will panic if the args cannot be serialized
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    #[inline]
    pub fn call_contract<T: BorshDeserialize>(
        &mut self,
        address: Address,
        function_name: &str,
        args: &[u8],
        max_units: Gas,
        value: u64,
    ) -> Result<T, ExternalCallError> {
        self.state_cache.flush();

        let bytes = self.host_accessor.call_contract(&CallContractArgs {
            address,
            function_name,
            args,
            max_units,
            value,
        });

        borsh::from_slice(&bytes).expect("failed to deserialize")
    }

    #[cfg(feature = "bindings")]
    #[must_use]
    pub fn to_extern(&mut self, args: ExternalCallArgs) -> ExternalCallContext<'_, Self> {
        ExternalCallContext {
            args,
            context: self,
        }
    }
}

#[cfg(feature = "test")]
impl Context {
    #[must_use]
    pub fn with_actor(actor: Address) -> Self {
        Self {
            contract_address: Address::default(),
            actor,
            height: 0,
            timestamp: 0,
            action_id: Id::default(),
            state_cache: Cache::new(),
            host_accessor: Accessor::new(),
        }
    }

    /// Mocks an external function call.
    /// # Panics
    /// Panics if serialization fails.
    pub fn mock_function_call<T, U>(
        &self,
        address: Address,
        function_name: &str,
        args: T,
        value: u64,
        result: U,
    ) where
        T: BorshSerialize,
        U: BorshSerialize,
    {
        use crate::host::CALL_FUNCTION_PREFIX;

        let args = &borsh::to_vec(&args).expect("error serializing result");

        let contract_args = CallContractArgs {
            address,
            function_name,
            args,
            value,
            max_units: 0,
        };

        let contract_args = borsh::to_vec(&(CALL_FUNCTION_PREFIX, contract_args))
            .expect("error serializing result");

        // serialize the result as Ok(result) to mimic host spec
        let result: Result<U, ExternalCallError> = Ok(result);
        let result = borsh::to_vec(&result).expect("error serializing result");
        self.host_accessor.state().put(&contract_args, result);
    }

    /// Mocks a deploy call.
    /// # Panics
    /// Panics if serialization fails.
    pub fn mock_deploy(&self, contract_id: Id, account_creation_data: &[u8]) -> Address {
        use crate::host::DEPLOY_PREFIX;

        let key = borsh::to_vec(&(DEPLOY_PREFIX, contract_id, account_creation_data))
            .expect("failed to serialize args");

        let val = self.host_accessor.new_deploy_address();

        self.host_accessor.state().put(&key, val.as_ref().to_vec());

        val
    }

    /// Sets the balance for the specified address
    #[cfg(feature = "test")]
    pub fn mock_set_balance(&self, account: Address, balance: u64) {
        self.host_accessor.set_balance(account, balance);
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

/// Arguments for an external call.
#[cfg_attr(feature = "debug", derive(Debug))]
#[derive(Clone, Copy)]
pub struct ExternalCallArgs {
    pub contract_address: Address,
    pub max_units: Gas,
    pub value: u64,
}

#[cfg(feature = "bindings")]
pub use external::*;

#[cfg(feature = "bindings")]
mod external {
    use super::{BorshDeserialize, Context, ExternalCallArgs, ExternalCallError};

    /// Special context that is passed to external contracts.
    #[allow(clippy::module_name_repetitions)]
    #[cfg_attr(feature = "debug", derive(Debug))]
    pub struct ExternalCallContext<'a, T = Context> {
        pub(super) args: ExternalCallArgs,
        pub(super) context: &'a mut T,
    }

    impl ExternalCallContext<'_> {
        /// Attempts to call a function `name` with `args` on the given contract. This method
        /// is used to call functions on external contracts.
        /// # Errors
        /// Returns a [`ExternalCallError`] if the call fails.
        /// # Panics
        /// Will panic if the args cannot be serialized
        /// # Safety
        /// The caller must ensure that `function_name` + `args` point to valid memory locations.
        pub fn call_function<T: BorshDeserialize>(
            self,
            function_name: &str,
            args: &[u8],
        ) -> Result<T, ExternalCallError> {
            let ExternalCallArgs {
                contract_address,
                max_units,
                value,
            } = self.args;

            self.context
                .call_contract(contract_address, function_name, args, max_units, value)
        }
    }
}
