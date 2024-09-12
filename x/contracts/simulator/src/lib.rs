// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use std::ffi::CString;

pub mod bindings;
pub mod state;

mod ffi {
    use super::bindings::{
        Address, CallContractResponse, CreateContractResponse, SimulatorCallContext,
    };
    use libc::c_char;

    #[link(name = "simulator")]
    extern "C" {
        #[link_name = "CreateContract"]
        pub fn create_contract(db: usize, path: *const c_char) -> CreateContractResponse;

        #[link_name = "CallContract"]
        pub fn call_contract(db: usize, ctx: *const SimulatorCallContext) -> CallContractResponse;

        #[link_name = "GetBalance"]
        pub fn get_balance(db: usize, account: Address) -> u64;

        #[link_name = "SetBalance"]
        pub fn set_balance(db: usize, account: Address, balance: u64);
    }
}

pub fn create_contract(
    state: &state::Mutable<'_>,
    contract_path: &str,
) -> bindings::CreateContractResponse {
    let contract_path = CString::new(contract_path).unwrap();
    let state_addr = state as *const _ as usize;
    // Call FFI function to create contract
    unsafe { ffi::create_contract(state_addr, contract_path.as_ptr()) }
}

pub fn call_contract(
    state: &state::Mutable<'_>,
    context: &bindings::SimulatorCallContext,
) -> bindings::CallContractResponse {
    let state_addr = state as *const _ as usize;

    unsafe { ffi::call_contract(state_addr, context) }
}

pub fn get_balance(state: &state::Mutable<'_>, account: bindings::Address) -> u64 {
    let state_addr = state as *const _ as usize;
    unsafe { ffi::get_balance(state_addr, account) }
}

pub fn set_balance(state: &state::Mutable<'_>, account: bindings::Address, balance: u64) {
    let state_addr = state as *const _ as usize;
    unsafe { ffi::set_balance(state_addr, account, balance) }
}
