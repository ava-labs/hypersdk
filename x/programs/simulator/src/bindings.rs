// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

#![allow(non_upper_case_globals)]
#![allow(non_camel_case_types)]
#![allow(non_snake_case)]
#![allow(dead_code)]

use std::ffi::CString;
use wasmlanche_sdk::Address as SdkAddress;

// include the generated bindings
// reference: https://rust-lang.github.io/rust-bindgen/tutorial-3.html
include!(concat!(env!("OUT_DIR"), "/bindings.rs"));

impl std::ops::Deref for Bytes {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        // # Safety:
        // These bytes were allocated by CGo
        // They are guaranteed to be valid for the length of the slice
        unsafe { std::slice::from_raw_parts(self.data, self.length as usize) }
    }
}

impl From<SdkAddress> for Address {
    fn from(value: SdkAddress) -> Self {
        Address {
            // # Safety:
            // Address is a simple wrapper around an array of bytes
            // this will fail at compile time if the size is changed
            address: unsafe {
                std::mem::transmute::<SdkAddress, [libc::c_uchar; SdkAddress::LEN]>(value)
            },
        }
    }
}

impl SimulatorCallContext {
    pub(crate) fn new(
        simulator: &super::Simulator,
        program: SdkAddress,
        method: &CString,
        params: &[u8],
        gas: u64,
    ) -> SimulatorCallContext {
        SimulatorCallContext {
            program_address: program.into(),
            actor_address: simulator.get_actor().into(),
            height: simulator.get_height(),
            timestamp: simulator.get_timestamp(),
            method: method.as_ptr(),
            params: Bytes {
                data: params.as_ptr(),
                length: params.len() as u64,
            },
            max_gas: gas,
        }
    }
}
