use std::{ffi::CStr, str::Utf8Error};
use libc::c_uint;
use std::ffi::CString;

pub use crate::{
    Address, Bytes, BytesWithError, CreateProgramResponse, Response,
    SimulatorCallContext,
};
use thiserror::Error;
use wasmlanche_sdk::ExternalCallError;
use wasmlanche_sdk::{Address as SdkAddress, Id};

#[derive(Error, Debug)]
pub enum SimulatorError {
    #[error("Error across the FFI boundary: {0}")]
    FFI(#[from] Utf8Error),
    #[error("Error from the response")]
    ResponseError,
    #[error(transparent)]
    Serialization(#[from] wasmlanche_sdk::borsh::io::Error),
    #[error(transparent)]
    ExternalCall(#[from] ExternalCallError),
}

impl Response {
    pub fn result<T>(&self) -> Result<T, SimulatorError>
    where
        T: wasmlanche_sdk::borsh::BorshDeserialize,
    {
        let bytes = self.result.get_slice();
        Ok(wasmlanche_sdk::borsh::from_slice(bytes)?)
    }
}

impl From<SdkAddress> for Address {
    fn from(value: SdkAddress) -> Self {
        Address {
            address: value.as_bytes().try_into().unwrap(),
        }
    }
}

impl SimulatorCallContext {
    pub fn new(program_address: SdkAddress, actor_address: SdkAddress, method: &CString, params: Vec<u8>, gas: u64) -> Self {
        SimulatorCallContext {
            program_address: program_address.into(),
            actor_address: actor_address.into(),
            height: 0,
            timestamp: 0,
            method: method.as_ptr(),
            params: params.as_ptr(),
            param_length: params.len() as c_uint,
            max_gas: gas as c_uint,
        }
    }
}

impl Bytes {
    pub fn get_slice(&self) -> &[u8] {
        unsafe { std::slice::from_raw_parts(self.data, self.length as usize) }
    }
}

impl CreateProgramResponse {
    pub fn program(&self) -> Result<SdkAddress, SimulatorError> {
        if self.has_error() {
            return Err(SimulatorError::ResponseError);
        };
        Ok(SdkAddress::new(self.program_address.address))
    }

    // TOOD: remove
    pub fn program_c_address(&self) -> Result<Address, SimulatorError> {
        if self.has_error() {
            return Err(SimulatorError::ResponseError);
        };
        Ok(self.program_address)
    }

    pub fn program_id(&self) -> Result<Id, SimulatorError> {
        if self.has_error() {
            return Err(SimulatorError::ResponseError);
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
        // need to make sure this pointer lives long enough
        let c_str = unsafe { CStr::from_ptr(self.error) };
        return c_str.to_str().map_err(SimulatorError::FFI);
    }
}

// impl fmt::Debug for CreateProgramResponse {
//     fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
//         f.debug_struct("CreateProgramResponse")
//             .field("program_address", &self.program())
//             .field("program_id", &self.program_id())
//             .field("error", &self.error())
//             .finish()
//     }
// }
