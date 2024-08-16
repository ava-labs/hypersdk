// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use libc::{c_char, c_uint};
use std::{
    ffi::{CStr, CString},
    fmt::Debug,
    str::Utf8Error,
};
use thiserror::Error;
use wasmlanche_sdk::{Address, ExternalCallError, Id};

use crate::{
    bindings::{Bytes, CallProgramResponse, CreateProgramResponse, SimulatorCallContext},
    state::{Mutable, SimpleState},
};

#[derive(Error, Debug)]
pub enum SimulatorError {
    #[error("Error across the FFI boundary: {0}")]
    Ffi(#[from] Utf8Error),
    #[error(transparent)]
    Serialization(#[from] wasmlanche_sdk::borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
    #[error("Error during program creation")]
    CreateProgram(String),
    #[error("Error during program execution")]
    CallProgram(String),
}

#[link(name = "simulator")]
extern "C" {
    #[link_name = "CreateProgram"]
    fn create_program(db: usize, path: *const c_char) -> CreateProgramResponse;

    #[link_name = "CallProgram"]
    fn call_program(db: usize, ctx: *const SimulatorCallContext) -> CallProgramResponse;

    #[link_name = "GetBalance"]
    fn get_balance(db: usize, account: Address) -> u64;

    #[link_name = "SetBalance"]
    fn set_balance(db: usize, account: Address, balance: u64);
}

pub struct Simulator<'a> {
    state: Mutable<'a>,
    actor: Address,
}

impl<'a> Simulator<'a> {
    /// Returns a new Simulator instance with the provided state and a default actor address.
    pub fn new(state: &'a mut SimpleState) -> Self {
        Simulator {
            state: Mutable::new(state),
            actor: Address::default(),
        }
    }

    /// Creates a new program from the given WASM binary path.
    pub fn create_program(&self, program_path: &str) -> CreateProgramResponse {
        let program_path = CString::new(program_path).unwrap();
        let state_addr = &self.state as *const _ as usize;
        // Call FFI function to create program
        unsafe { create_program(state_addr, program_path.as_ptr()) }
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
    pub fn call_program<T: wasmlanche_sdk::borsh::BorshSerialize>(
        &self,
        program: Address,
        method: &str,
        params: T,
        gas: u64,
    ) -> CallProgramResponse {
        // serialize the params
        let params = wasmlanche_sdk::borsh::to_vec(&params).expect("error serializing result");
        let method = CString::new(method).expect("Unable to create a cstring");
        // build the call context
        let context = SimulatorCallContext::new(program, self.actor, &method, &params, gas);
        let state_addr = &self.state as *const _ as usize;

        unsafe { call_program(state_addr, &context) }
    }

    /// Returns the actor address for the simulator.
    pub fn get_actor(&self) -> Address {
        self.actor
    }

    /// Sets the actor address for the simulator.
    pub fn set_actor(&mut self, actor: Address) {
        self.actor = actor;
    }

    /// Returns the balance of the given account.
    pub fn get_balance(&self, account: Address) -> u64 {
        let state_addr = &self.state as *const _ as usize;
        unsafe { get_balance(state_addr, account) }
    }

    /// Sets the balance of the given account.
    pub fn set_balance(&mut self, account: Address, balance: u64) {
        let state_addr = &self.state as *const _ as usize;
        unsafe { set_balance(state_addr, account, balance) }
    }
}

impl CreateProgramResponse {
    /// Returns the program address, which uniquely identifies an instance of the program.
    pub fn program(&self) -> Result<Address, SimulatorError> {
        if self.has_error() {
            let error = self.error()?;
            return Err(SimulatorError::CreateProgram(error.into()));
        };
        Ok(Address::new(self.program_address.address))
    }

    /// Returns the program ID, which uniquely identifies the program's bytecode.
    ///
    /// Multiple program addresses can reference the same program ID, similar to
    /// how multiple instances of a smart contract can share the same bytecode.
    pub fn program_id(&self) -> Result<Id, SimulatorError> {
        if self.has_error() {
            let error = self.error()?;
            return Err(SimulatorError::CreateProgram(error.into()));
        };
        Ok(self.program_id.id)
    }

    /// Returns the error message if the program creation failed.
    pub fn has_error(&self) -> bool {
        !self.error.is_null()
    }

    /// Returns the error message if the program creation failed.
    pub fn error(&self) -> Result<&str, SimulatorError> {
        if !self.has_error() {
            return Ok("");
        }
        let c_str = unsafe { CStr::from_ptr(self.error) };
        return c_str.to_str().map_err(SimulatorError::Ffi);
    }

    /// This function panics if the response contains an error.
    /// This is useful for testing.
    ///
    /// # Panics
    /// Panics if the response contains an error.
    pub fn unwrap(&self) {
        if self.has_error() {
            panic!("CreateProgramResponse errored")
        }
    }
}

impl CallProgramResponse {
    /// Returns the deserialized result of the program call.
    ///
    /// # Returns
    /// `Result<T, SimulatorError>` where T is the expected return type
    pub fn result<T>(&self) -> Result<T, SimulatorError>
    where
        T: wasmlanche_sdk::borsh::BorshDeserialize,
    {
        if self.has_error() {
            let error = self.error()?;
            return Err(SimulatorError::CallProgram(error.into()));
        };

        Ok(wasmlanche_sdk::borsh::from_slice(&self.result)?)
    }

    /// Returns whether the program call resulted in an error.
    pub fn has_error(&self) -> bool {
        !self.error.is_null()
    }

    /// Returns the error message if there was one.
    pub fn error(&self) -> Result<&str, SimulatorError> {
        if !self.has_error() {
            return Ok("");
        }
        let c_str = unsafe { CStr::from_ptr(self.error) };
        return c_str.to_str().map_err(SimulatorError::Ffi);
    }

    /// This function panics if the response contains an error.
    ///
    /// # Panics
    /// Panics if the response contains an error.
    pub fn unwrap(&self) {
        if self.has_error() {
            panic!("CallProgramResponse errored")
        }
    }
}

impl SimulatorCallContext {
    pub fn new(
        program_address: Address,
        actor_address: Address,
        method: &CString,
        params: &[u8],
        gas: u64,
    ) -> Self {
        SimulatorCallContext {
            program_address: program_address.into(),
            actor_address: actor_address.into(),
            height: 0,
            timestamp: 0,
            method: method.as_ptr(),
            params: Bytes {
                data: params.as_ptr(),
                length: params.len() as c_uint,
            },
            max_gas: gas as c_uint,
        }
    }
}
