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
    fn CreateProgram(db: usize, path: *const c_char) -> CreateProgramResponse;

    fn CallProgram(db: usize, ctx: *const SimulatorCallContext) -> CallProgramResponse;
}

pub struct Simulator<'a> {
    state: Mutable<'a>,
    pub actor: Address,
}

impl<'a> Simulator<'a> {
    pub fn new(state: &'a mut SimpleState) -> Self {
        Simulator {
            state: Mutable::new(state),
            actor: Address::default(),
        }
    }

    pub fn create_program(&self, program_path: &str) -> CreateProgramResponse {
        let program_path = CString::new(program_path).unwrap();
        let state_addr = &self.state as *const _ as usize;
        unsafe { CreateProgram(state_addr, program_path.as_ptr()) }
    }

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

        unsafe { CallProgram(state_addr, &context) }
    }
}

impl<'a> From<&Mutable<'a>> for *mut Mutable<'a> {
    fn from(state: &Mutable) -> Self {
        state as *const Mutable as *mut Mutable
    }
}

impl CreateProgramResponse {
    pub fn program(&self) -> Result<Address, SimulatorError> {
        if self.has_error() {
            let error = self.error()?;
            return Err(SimulatorError::CreateProgram(error.into()));
        };
        Ok(Address::new(self.program_address.address))
    }

    pub fn program_id(&self) -> Result<Id, SimulatorError> {
        if self.has_error() {
            let error = self.error()?;
            return Err(SimulatorError::CreateProgram(error.into()));
        };
        Ok(self.program_id.id)
    }

    pub fn has_error(&self) -> bool {
        !self.error.is_null()
    }

    // get error
    pub fn error(&self) -> Result<&str, SimulatorError> {
        if !self.has_error() {
            return Ok("");
        }
        let c_str = unsafe { CStr::from_ptr(self.error) };
        return c_str.to_str().map_err(SimulatorError::Ffi);
    }

    // will panic if there is an error. helpful for testing
    pub fn unwrap(&self) {
        if self.has_error() {
            panic!("CreateProgramResponse errored")
        }
    }
}

impl CallProgramResponse {
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

    pub fn has_error(&self) -> bool {
        !self.error.is_null()
    }

    // get error
    pub fn error(&self) -> Result<&str, SimulatorError> {
        if !self.has_error() {
            return Ok("");
        }
        let c_str = unsafe { CStr::from_ptr(self.error) };
        return c_str.to_str().map_err(SimulatorError::Ffi);
    }

    // will panic if there is an error. helpful for testing
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
