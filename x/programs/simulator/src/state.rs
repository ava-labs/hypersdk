use crate::bindings::{Bytes, BytesWithError};
use libc::c_char;
use std::{collections::HashMap, ffi::CString};

// define constant error messages
// TODO: Would love a less-hardcodey way of representing errors between rust <-> go
pub const ERR_NOT_FOUND: &str = "not found";

#[repr(transparent)]
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

// We re-define this mutable in rust for more control over the pointer types
// mute clippy warnings
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

pub extern "C" fn get_state_callback(obj_ptr: *mut SimpleState, key: Bytes) -> BytesWithError {
    let obj = unsafe { &mut *obj_ptr };
    let value = obj.get_value(&key);

    match value {
        Some(v) => BytesWithError {
            bytes: Bytes {
                data: v.as_ptr(),
                length: v.len() as u32,
            },
            error: std::ptr::null(),
        },
        None => BytesWithError {
            bytes: Bytes {
                data: std::ptr::null_mut(),
                length: 0,
            },
            error: CString::new(ERR_NOT_FOUND).unwrap().into_raw(),
        },
    }
}

pub extern "C" fn insert_state_callback(
    obj_ptr: *mut SimpleState,
    key: Bytes,
    value: Bytes,
) -> *const c_char {
    let obj = unsafe { &mut *obj_ptr };
    obj.insert(key.to_vec(), value.to_vec());
    std::ptr::null()
}

pub extern "C" fn remove_state_callback(obj_ptr: *mut SimpleState, key: Bytes) -> *const c_char {
    let obj = unsafe { &mut *obj_ptr };
    obj.remove(key.to_vec());
    std::ptr::null()
}

pub type GetStateCallback =
    extern "C" fn(simObjectPtr: *mut SimpleState, key: Bytes) -> BytesWithError;
pub type InsertStateCallback =
    extern "C" fn(objectPtr: *mut SimpleState, key: Bytes, value: Bytes) -> *const c_char;
pub type RemoveStateCallback =
    extern "C" fn(objectPtr: *mut SimpleState, key: Bytes) -> *const c_char;
