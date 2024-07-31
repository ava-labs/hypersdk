use std::ffi::CString;

use libc::c_char;

use crate::{state::{Mutable, SimpleState}, types::CreateProgramResponse};


pub struct Simulator {
    state: Mutable
}

impl Simulator {
    pub fn new() -> Self {
        Simulator {
            state: Mutable::new()
        }
    }

    pub fn CallProgramTest(&self) {
        unsafe {
            CallProgram((&self.state).into())
        }
    }

    pub fn CreateProgram(&self, program_path: &str) -> CreateProgramResponse {
        // TODO: do we need to free this?
        let program_path = CString::new(program_path).unwrap();
        unsafe {
            CreateProgram((&self.state).into(), program_path.as_ptr())
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
}