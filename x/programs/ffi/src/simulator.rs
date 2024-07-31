use std::ffi::CString;

use libc::c_char;

use crate::{state::{Mutable, SimpleState}, types::{CreateProgramResponse, ExecutionRequest, Response, SimulatorContext}};


pub struct Simulator {
    state: Mutable
}

impl Simulator {
    pub fn new() -> Self {
        Simulator {
            state: Mutable::new()
        }
    }

    pub fn call_program_test(&self) {
        unsafe {
            CallProgram((&self.state).into())
        }
    }

    pub fn create_program(&self, program_path: &str) -> CreateProgramResponse {
        // TODO: do we need to free this?
        let program_path = CString::new(program_path).unwrap();
        unsafe {
            CreateProgram((&self.state).into(), program_path.as_ptr())
        }
    }

    pub fn execute(&self, context: &SimulatorContext, request: &ExecutionRequest) -> Response {
        unsafe {
            Execute((&self.state).into(), context, request)
        }
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
    fn Execute(db: *mut Mutable, ctx: *const SimulatorContext, request: *const ExecutionRequest) -> Response;

}