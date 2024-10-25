// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{borsh, Address};
use core::{marker::PhantomData, ops::Deref};
use simulator::{
    bindings::{Address as BindingAddress, Bytes, SimulatorCallContext},
    state::{self, Mutable},
};
use std::{
    error::Error as StdError,
    ffi::{c_uchar, CStr, CString},
    fmt::{Debug, Display},
    str::Utf8Error,
};

pub use state::SimpleState;

#[derive(thiserror::Error, Debug)]
pub enum Error {
    #[error("Error across the FFI boundary: {0}")]
    Ffi(#[from] Utf8Error),
    #[error(transparent)]
    Serialization(#[from] borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
    #[error("Error during contract creation")]
    CreateContract(String),
    #[error("Error during contract execution")]
    CallContract(String),
}

pub struct ExternalCallError(crate::ExternalCallError);

impl From<crate::ExternalCallError> for ExternalCallError {
    fn from(e: crate::ExternalCallError) -> Self {
        Self(e)
    }
}

impl Display for ExternalCallError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        Display::fmt(&self.0, f)
    }
}

impl Debug for ExternalCallError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        Debug::fmt(&self.0, f)
    }
}

impl StdError for ExternalCallError {}

impl From<Address> for BindingAddress {
    fn from(value: Address) -> Self {
        Self {
            // # Safety:
            // Address is a simple wrapper around an array of bytes
            // this will fail at compile time if the size is changed
            address: unsafe { std::mem::transmute::<Address, [c_uchar; Address::LEN]>(value) },
        }
    }
}

struct CallContext<'a, 'b> {
    simulator: &'a Simulator<'b>,
    contract: Address,
    method: CString,
    params: Vec<u8>,
    gas: u64,
}

impl<'a, 'b> CallContext<'a, 'b> {
    fn new(
        simulator: &'a Simulator<'b>,
        contract: Address,
        method: CString,
        params: Vec<u8>,
        gas: u64,
    ) -> Self {
        Self {
            simulator,
            contract,
            method,
            params,
            gas,
        }
    }
}

struct BorrowedCallContext<'a>(SimulatorCallContext, PhantomData<&'a usize>);

impl<'a> From<&'a CallContext<'_, '_>> for BorrowedCallContext<'a> {
    fn from(ctx: &'a CallContext<'_, '_>) -> Self {
        Self(
            SimulatorCallContext {
                contract_address: ctx.contract.into(),
                actor_address: ctx.simulator.get_actor().into(),
                height: ctx.simulator.get_height(),
                timestamp: ctx.simulator.get_timestamp(),
                method: ctx.method.as_ptr(),
                params: Bytes {
                    data: ctx.params.as_ptr(),
                    length: ctx.params.len(),
                },
                max_gas: ctx.gas,
            },
            PhantomData,
        )
    }
}

impl Deref for BorrowedCallContext<'_> {
    type Target = SimulatorCallContext;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

pub struct Simulator<'a> {
    state: Mutable<'a>,
    actor: Address,
    height: u64,
    timestamp: u64,
}

impl<'a> Simulator<'a> {
    /// Returns a new Simulator instance with the provided state and a default actor address.
    pub fn new(state: &'a mut SimpleState) -> Self {
        Simulator {
            state: Mutable::new(state),
            actor: Address::default(),
            height: 0,
            timestamp: 0,
        }
    }

    /// Creates a new contract from the given WASM binary path.
    /// # Errors
    /// Returns an error if the contract creation fails.
    pub fn create_contract(&self, contract_path: &str) -> Result<CreateContractResult, Error> {
        let result = simulator::create_contract(&self.state, contract_path);

        if !result.error.is_null() {
            let error = {
                let c_str = unsafe { CStr::from_ptr(result.error) };
                c_str.to_str().map_err(Error::Ffi)?
            };

            return Err(Error::CreateContract(error.into()));
        }
        let address = Address::new(result.contract_address.address);
        let id = result.contract_id.to_vec().into_boxed_slice();

        Ok(CreateContractResult { id, address })
    }

    /// Calls a contract with specified method, parameters, and gas limit.
    ///
    /// # Parameters
    /// - `params`: Borsh-serializable tuple. Exclude context for public functions.
    ///   For single values, use `(param,)`. Specify types if not explicit.
    ///   Example: `(param1 as u64, param2 as u64)`
    ///
    /// # Errors
    /// returns an error if the either the call or deserialization fails.
    ///
    /// # Panics
    /// Panics if the params fail to serialize.
    pub fn call_contract<T, U>(
        &self,
        contract: Address,
        method: &str,
        params: U,
        gas: u64,
    ) -> Result<T, Error>
    where
        T: borsh::BorshDeserialize,
        U: borsh::BorshSerialize,
    {
        let method = CString::new(method).expect("error converting method to CString");
        let params = borsh::to_vec(&params).expect("error serializing result");

        let context = CallContext::new(self, contract, method, params, gas);
        let context = BorrowedCallContext::from(&context);

        let result = simulator::call_contract(&self.state, &context);

        if !result.error.is_null() {
            let error = {
                let c_str = unsafe { CStr::from_ptr(result.error) };
                c_str.to_str().map_err(Error::Ffi)?
            };

            return Err(Error::CallContract(error.into()));
        };

        Ok(borsh::from_slice(&result.result)?)
    }

    /// Returns the actor address for the simulator.
    #[must_use]
    pub fn get_actor(&self) -> Address {
        self.actor
    }

    /// Sets the actor address for the simulator.
    pub fn set_actor(&mut self, actor: Address) {
        self.actor = actor;
    }

    /// Returns the height of the blockchain.
    #[must_use]
    pub fn get_height(&self) -> u64 {
        self.height
    }

    /// Sets the height of the blockchain.
    pub fn set_height(&mut self, height: u64) {
        self.height = height;
    }

    /// Returns the timestamp of the blockchain.
    #[must_use]
    pub fn get_timestamp(&self) -> u64 {
        self.timestamp
    }

    /// Sets the timestamp of the blockchain.
    pub fn set_timestamp(&mut self, timestamp: u64) {
        self.timestamp = timestamp;
    }

    /// Returns the balance of the given account.
    #[must_use]
    pub fn get_balance(&self, account: Address) -> u64 {
        simulator::get_balance(&self.state, account.into())
    }

    /// Sets the balance of the given account.
    pub fn set_balance(&mut self, account: Address, balance: u64) {
        simulator::set_balance(&self.state, account.into(), balance);
    }
}

pub struct CreateContractResult {
    pub id: Box<[u8]>,
    pub address: Address,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn initial_balance_is_zero() {
        let mut state = SimpleState::new();
        let simulator = Simulator::new(&mut state);
        let alice = Address::new([1; 33]);

        let bal = simulator.get_balance(alice);
        assert_eq!(bal, 0);
    }

    #[test]
    fn get_balance() {
        let balance_manager_prefix = [0x01];
        let alice = Address::new([1; 33]);
        let mut state = SimpleState::new();
        let exptected_balance = 999u64;

        let key = balance_manager_prefix
            .into_iter()
            .chain(alice.as_ref().iter().copied())
            .chain(b"balance".iter().copied())
            .collect();

        state.insert(key, Box::from(exptected_balance.to_be_bytes()));

        let simulator = Simulator::new(&mut state);

        let bal = simulator.get_balance(alice);
        assert_eq!(bal, exptected_balance);
    }

    #[test]
    fn set_balance() {
        let expected_balance = 100;
        let mut state = SimpleState::new();
        let mut simulator = Simulator::new(&mut state);
        let alice = Address::new([1; 33]);

        simulator.set_balance(alice, expected_balance);
        let bal = simulator.get_balance(alice);
        assert_eq!(bal, expected_balance);
    }
}
