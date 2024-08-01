use std::ffi::CString;

use libc::{c_char, c_uint};
use wasmlanche_sdk::Address;

use crate::{
    state::Mutable,
    types::{CreateProgramResponse, ExecutionRequest, Response, SimulatorCallContext},
};

pub struct Simulator {
    state: Mutable,
    // TODO: create a map (string -> address) to improve dev ux with addresses
    pub actor: Address,
}

impl Simulator {
    pub fn new() -> Self {
        Simulator {
            state: Mutable::new(),
            actor: Address::default(),
        }
    }

    pub fn call_program_test(&self) {
        unsafe { CallProgram((&self.state).into()) }
    }

    pub fn create_program(&self, program_path: &str) -> CreateProgramResponse {
        // TODO: do we need to free this?
        let program_path = CString::new(program_path).unwrap();
        unsafe { CreateProgram((&self.state).into(), program_path.as_ptr()) }
    }

    pub fn execute<T: wasmlanche_sdk::borsh::BorshSerialize>(
        &self,
        program: Address,
        method: &str,
        params: T,
        gas: u64,
    ) -> Response {
        // build the call context
        let context = SimulatorCallContext::new(program, self.actor);
        // build the executrion request
        let method = CString::new(method).expect("Unable to create a cstring");
        // serialize the params
        let params = wasmlanche_sdk::borsh::to_vec(&params).expect("error serializing result");

        let request = ExecutionRequest {
            method: method.as_ptr(),
            params: params.as_ptr(),
            param_length: params.len() as c_uint,
            max_gas: gas as c_uint,
        };

        unsafe { Execute((&self.state).into(), &context, &request) }
    }
}

impl From<&Mutable> for *mut Mutable {
    fn from(state: &Mutable) -> Self {
        state as *const Mutable as *mut Mutable
    }
}

#[link(name = "simulator", kind = "dylib")]
extern "C" {
    fn CallProgram(db: *mut Mutable);
    fn CreateProgram(db: *mut Mutable, path: *const c_char) -> CreateProgramResponse;
    fn Execute(
        db: *mut Mutable,
        ctx: *const SimulatorCallContext,
        request: *const ExecutionRequest,
    ) -> Response;
}
