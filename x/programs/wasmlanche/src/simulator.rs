// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{borsh, Address, ProgramId};
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
    #[error("Error during program creation")]
    CreateProgram(String),
    #[error("Error during program execution")]
    CallProgram(String),
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
    program: Address,
    method: CString,
    params: Vec<u8>,
    gas: u64,
}

impl<'a, 'b> CallContext<'a, 'b> {
    fn new(
        simulator: &'a Simulator<'b>,
        program: Address,
        method: CString,
        params: Vec<u8>,
        gas: u64,
    ) -> Self {
        Self {
            simulator,
            program,
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
                program_address: ctx.program.into(),
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

    /// Creates a new program from the given WASM binary path.
    #[must_use]
    pub fn create_program(&self, program_path: &str) -> CreateProgramResponse {
        simulator::create_program(&self.state, program_path).into()
    }

    /// Calls a program with specified method, parameters, and gas limit.
    ///
    /// # Parameters
    /// - `params`: Borsh-serializable tuple. Exclude context for public functions.
    ///   For single values, use `(param,)`. Specify types if not explicit.
    ///   Example: `(param1 as u64, param2 as u64)`
    ///
    /// # Returns
    /// `CallProgramResponse` with:
    /// - `result<R>()`: Call result (specify type `R`)
    /// - `error()` or `has_error()`: Error information
    ///
    /// # Panics
    /// Panics if the params fail to serialize.
    pub fn call_program<T>(
        &self,
        program: Address,
        method: &str,
        params: T,
        gas: u64,
    ) -> CallProgramResponse
    where
        T: borsh::BorshSerialize,
    {
        let method = CString::new(method).expect("error converting method to CString");
        let params = borsh::to_vec(&params).expect("error serializing result");

        let context = CallContext::new(self, program, method, params, gas);
        let context = BorrowedCallContext::from(&context);

        simulator::call_program(&self.state, &context).into()
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

pub struct CreateProgramResponse(simulator::bindings::CreateProgramResponse);

impl From<simulator::bindings::CreateProgramResponse> for CreateProgramResponse {
    fn from(value: simulator::bindings::CreateProgramResponse) -> Self {
        Self(value)
    }
}

impl CreateProgramResponse {
    /// Returns the program address, which uniquely identifies an instance of the program.
    /// # Errors
    /// Returns an error if the program creation failed.
    pub fn program(&self) -> Result<Address, Error> {
        if self.has_error() {
            let error = self.error()?;
            return Err(Error::CreateProgram(error.into()));
        }

        Ok(Address::new(self.0.program_address.address))
    }

    /// Returns the program ID, which uniquely identifies the program's bytecode.
    ///
    /// Multiple program addresses can reference the same program ID, similar to
    /// how multiple instances of a smart contract can share the same bytecode.
    /// # Errors
    /// Returns an error if the program creation failed.
    pub fn program_id(&self) -> Result<ProgramId, Error> {
        if self.has_error() {
            let error = self.error()?;
            return Err(Error::CreateProgram(error.into()));
        }

        // TODO:
        // This should give back a borrowed ProgramId
        // we need to differentiate between the two types
        // we should also explore the ability to deserialize
        // borrowed types for the public API of programs
        Ok(Box::<[u8]>::from(self.0.program_id).into())
    }

    /// Returns the error message if the program creation failed.
    #[must_use]
    pub fn has_error(&self) -> bool {
        !self.0.error.is_null()
    }

    /// Returns the error message if the program creation failed.
    /// # Errors
    /// Returns an error if the error message is not valid UTF-8.
    pub fn error(&self) -> Result<&str, Error> {
        if !self.has_error() {
            return Ok("");
        }
        let c_str = unsafe { CStr::from_ptr(self.0.error) };
        return c_str.to_str().map_err(Error::Ffi);
    }

    /// This function panics if the response contains an error.
    /// This is useful for testing.
    ///
    /// # Panics
    /// Panics if the response contains an error.
    pub fn unwrap(&self) {
        assert!(!self.has_error(), "CreateProgramResponse errored");
    }
}

#[derive(Debug)]
pub struct CallProgramResponse(simulator::bindings::CallProgramResponse);

impl From<simulator::bindings::CallProgramResponse> for CallProgramResponse {
    fn from(value: simulator::bindings::CallProgramResponse) -> Self {
        Self(value)
    }
}

impl CallProgramResponse {
    /// Returns the deserialized result of the program call.
    ///
    /// # Returns
    /// `Result<T, SimulatorError>` where T is the expected return type
    /// # Errors
    /// Returns an error if the program call failed or if the result could not be deserialized.
    pub fn result<T>(&self) -> Result<T, Error>
    where
        T: borsh::BorshDeserialize,
    {
        if self.has_error() {
            let error = self.error()?;
            return Err(Error::CallProgram(error.into()));
        };

        Ok(borsh::from_slice(&self.0.result)?)
    }

    /// Returns whether the program call resulted in an error.
    #[must_use]
    pub fn has_error(&self) -> bool {
        !self.0.error.is_null()
    }

    /// Returns the error message if there was one.
    /// # Errors
    /// Returns an error if calling the program failed
    /// or if the error message is not valid UTF-8.
    pub fn error(&self) -> Result<&str, Error> {
        if !self.has_error() {
            return Ok("");
        }
        let c_str = unsafe { CStr::from_ptr(self.0.error) };
        return c_str.to_str().map_err(Error::Ffi);
    }

    /// This function panics if the response contains an error.
    ///
    /// # Panics
    /// Panics if the response contains an error.
    pub fn unwrap(&self) {
        assert!(!self.has_error(), "CallProgramResponse errored");
    }
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
        let account_data_prefix = [0x00];
        let account_prefix = [0x01];
        let alice = Address::new([1; 33]);
        let mut state = SimpleState::new();
        let exptected_balance = 999u64;

        let key = account_prefix
            .into_iter()
            .chain(alice.as_ref().iter().copied())
            .chain(account_data_prefix)
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
