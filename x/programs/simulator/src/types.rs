use std::{ffi::CStr, str::Utf8Error};

use libc::{c_char, c_int, c_uchar, c_uint};
use std::fmt;
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

#[repr(C)]
pub struct ExecutionRequest {
    pub method: *const c_char,
    pub params: *const c_uchar,
    pub param_length: c_uint,
    pub max_gas: c_uint,
}

#[repr(C)]
pub struct Response {
    // Todo: remove id
    pub id: c_int,
    // string error message
    pub error: *const c_char,
    // result byte array
    pub result: Bytes,
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

#[repr(C)]
struct ID {
    pub id: [c_uchar; 32],
}

#[derive(Clone, Copy)]
#[repr(C)]
pub struct Address {
    pub address: [c_uchar; 33],
}

impl From<SdkAddress> for Address {
    fn from(value: SdkAddress) -> Self {
        Address {
            address: value.as_bytes().try_into().unwrap(),
        }
    }
}

#[repr(C)]
pub struct SimulatorCallContext {
    program_address: Address,
    actor_address: Address,
    height: c_uint,
    timestamp: c_uint,
}

impl SimulatorCallContext {
    pub fn new(program_address: SdkAddress, actor_address: SdkAddress) -> Self {
        SimulatorCallContext {
            program_address: program_address.into(),
            actor_address: actor_address.into(),
            height: 0,
            timestamp: 0,
        }
    }
}

#[repr(C)]
pub struct Bytes {
    pub data: *mut u8,
    pub len: usize,
}

#[repr(C)]
pub struct BytesWithError {
    pub data: *mut u8,
    pub len: usize,
    pub error: *const c_char,
}

impl Bytes {
    pub fn get_slice(&self) -> &[u8] {
        unsafe { std::slice::from_raw_parts(self.data, self.len) }
    }
}

#[repr(C)]
pub struct CreateProgramResponse {
    program_address: Address,
    program_id: ID,
    error: *const c_char,
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

impl fmt::Debug for CreateProgramResponse {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("CreateProgramResponse")
            .field("program_address", &self.program())
            .field("program_id", &self.program_id())
            .field("error", &self.error())
            .finish()
    }
}

// impl fmt::Debug for Response {
//     fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt:: Result {
//         f.debug
//     }
// }
