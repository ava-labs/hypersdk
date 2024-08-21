// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::bindings::{Bytes, BytesWithError};
use std::{collections::HashMap, ffi::{CStr, CString}};

// define constant error messages
// TODO: Would love a less-hardcodey way of representing errors between rust <-> go
pub const ERR_NOT_FOUND: &str = "not found";

/// A simple key-value store representing the state of the simulated VM.
#[repr(transparent)]
#[derive(Debug)]
pub struct SimpleState {
    state: HashMap<Vec<u8>, Vec<u8>>,
}

impl SimpleState {
    pub fn new() -> SimpleState {
        SimpleState {
            state: HashMap::new(),
        }
    }
    pub fn get_value(&self, key: &[u8]) -> Option<&Vec<u8>> {
        self.state.get(key)
    }

    pub fn insert(&mut self, key: Vec<u8>, value: Vec<u8>) {
        self.state.insert(key, value);
    }

    pub fn remove(&mut self, key: Vec<u8>) {
        self.state.remove(&key);
    }
}
impl Default for SimpleState {
    fn default() -> Self {
        Self::new()
    }
}

#[repr(C)]
pub struct Mutable<'a> {
    pub state: &'a mut SimpleState,
    pub get_state: GetStateCallback,
    pub insert_state: InsertStateCallback,
    pub remove_state: RemoveStateCallback,
}

impl<'a> Mutable<'a> {
    pub fn new(state: &'a mut SimpleState) -> Self {
        Mutable {
            state,
            get_state: get_state_callback,
            insert_state: insert_state_callback,
            remove_state: remove_state_callback,
        }
    }
}

pub extern "C" fn get_state_callback(state: &mut SimpleState, key: Bytes) -> BytesWithError {
    match state.get_value(&key) {
        Some(v) => BytesWithError {
            bytes: Bytes {
                data: v.as_ptr(),
                length: v.len(),
            },
            error: std::ptr::null(),
        },
        None => BytesWithError {
            bytes: Bytes {
                data: std::ptr::null_mut(),
                length: 0,
            },
            error: CStr::from_bytes_with_nul(bytes!(ERR_NOT_FOUND)).unwrap().as_ptr(),
        },
    }
}

pub extern "C" fn insert_state_callback(state: &mut SimpleState, key: Bytes, value: Bytes) {
    state.insert(key.to_vec(), value.to_vec());
}

pub extern "C" fn remove_state_callback(state: &mut SimpleState, key: Bytes) {
    state.remove(key.to_vec());
}

/// Callback functions for retrieving/modifying state values.
///
/// These function are used as part of the FFI interface to allow CGO code
/// to query the Rust-side state storage.
pub type GetStateCallback =
    extern "C" fn(simObjectPtr: &mut SimpleState, key: Bytes) -> BytesWithError;
pub type InsertStateCallback = extern "C" fn(objectPtr: &mut SimpleState, key: Bytes, value: Bytes);
pub type RemoveStateCallback = extern "C" fn(objectPtr: &mut SimpleState, key: Bytes);
