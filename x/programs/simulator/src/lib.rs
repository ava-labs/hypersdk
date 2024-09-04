// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::ffi::CString;

pub mod bindings;
pub mod state;

mod ffi {
    use super::bindings::{
        Address, CallProgramResponse, CreateProgramResponse, SimulatorCallContext,
    };
    use libc::c_char;

    #[link(name = "simulator")]
    extern "C" {
        #[link_name = "CreateProgram"]
        pub fn create_program(db: usize, path: *const c_char) -> CreateProgramResponse;

        #[link_name = "CallProgram"]
        pub fn call_program(db: usize, ctx: *const SimulatorCallContext) -> CallProgramResponse;

        #[link_name = "GetBalance"]
        pub fn get_balance(db: usize, account: Address) -> u64;

        #[link_name = "SetBalance"]
        pub fn set_balance(db: usize, account: Address, balance: u64);
    }
}

pub fn create_program(
    state: &state::Mutable<'_>,
    program_path: &str,
) -> bindings::CreateProgramResponse {
    let program_path = CString::new(program_path).unwrap();
    let state_addr = state as *const _ as usize;
    // Call FFI function to create program
    unsafe { ffi::create_program(state_addr, program_path.as_ptr()) }
}

pub fn call_program(
    state: &state::Mutable<'_>,
    context: &bindings::SimulatorCallContext,
) -> bindings::CallProgramResponse {
    let state_addr = state as *const _ as usize;

    unsafe { ffi::call_program(state_addr, context) }
}

pub fn get_balance(state: &state::Mutable<'_>, account: bindings::Address) -> u64 {
    let state_addr = state as *const _ as usize;
    unsafe { ffi::get_balance(state_addr, account) }
}

pub fn set_balance(state: &state::Mutable<'_>, account: bindings::Address, balance: u64) {
    let state_addr = state as *const _ as usize;
    unsafe { ffi::set_balance(state_addr, account, balance) }
}
