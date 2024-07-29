
use libc::{c_char, c_int, c_uchar, c_uint};

#[repr(C)]
pub struct ExecutionRequest {
    pub method: *const c_char,
    pub params: *const c_uchar,
    pub param_length: c_uint,
    pub max_gas: c_uint,
}

#[repr(C)]
pub struct Response {
    pub id: c_int,
    // string error message
    pub error: *const c_char,
    // result byte array
    pub result: *const c_uchar,
}

// this represents the db state
// aka state.SimpleMutable
#[repr(C)]
pub struct SimpleMutable {
    pub value: c_int,
}

#[repr(C)]
pub struct ID {
    pub id: [c_uchar; 32],
}

#[repr(C)]
pub struct Address {
    pub address: [c_uchar; 33],
}

#[repr(C)]
pub struct SimulatorContext {
    pub program_address: Address,
    pub actor_address: Address,
    pub height: c_uint,
    pub timestamp: c_uint,
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