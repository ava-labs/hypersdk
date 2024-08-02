use std::ffi::CString;

use libc::c_char;
use wasmlanche_sdk::Address;

use crate::{
    state::Mutable,
    types::{CreateProgramResponse, CallProgramResponse, SimulatorCallContext},
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

    pub fn create_program(&self, program_path: &str) -> CreateProgramResponse {
        // TODO: do we need to free this?
        let program_path = CString::new(program_path).unwrap();
        unsafe { CreateProgram((&self.state).into(), program_path.as_ptr()) }
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
        let context = SimulatorCallContext::new(program, self.actor, &method, params, gas);

        unsafe { CallProgram((&self.state).into(), &context) }
    }
}

impl From<&Mutable> for *mut Mutable {
    fn from(state: &Mutable) -> Self {
        state as *const Mutable as *mut Mutable
    }
}

#[link(name = "simulator")]
extern "C" {
    fn CreateProgram(db: *mut Mutable, path: *const c_char) -> CreateProgramResponse;
    fn CallProgram(
        db: *mut Mutable,
        ctx: *const SimulatorCallContext,
    ) -> CallProgramResponse;
}
