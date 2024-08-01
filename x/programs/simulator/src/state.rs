use std::{collections::HashMap, ffi::CString};

use libc::c_char;

use crate::types::{Bytes, BytesWithError};

#[repr(C)]
pub struct SimpleState {
    state: HashMap<Vec<u8>, Vec<u8>>,
}

impl SimpleState {
    pub fn new() -> SimpleState {
        SimpleState {
            state: HashMap::new(),
        }
    }
    pub fn get_value(&self, key: &Vec<u8>) -> Option<&Vec<u8>> {
        self.state.get(key)
    }

    pub fn insert(&mut self, key: Vec<u8>, value: Vec<u8>) {
        self.state.insert(key, value);
    }

    pub fn remove(&mut self, key: Vec<u8>) {
        self.state.remove(&key);
    }
}

#[repr(C)]
pub struct Mutable {
    pub obj: *mut SimpleState,
    pub get_state: GetStateCallback,
    pub insert_state: InsertStateCallback,
    pub remove_state: RemoveStateCallback,
    // TODO: why does this need to be in the bottom?
    pub state: Box<SimpleState>,
}

impl Mutable {
    pub fn new() -> Self {
        let mut state = Box::new(SimpleState::new());
        let obj = Box::as_mut(&mut state) as *mut SimpleState;
        Mutable {
            state: state,
            obj: obj,
            get_state: get_state_callback,
            insert_state: insert_state_callback,
            remove_state: remove_state_callback,
        }
    }
}

pub extern "C" fn get_state_callback(obj_ptr: *mut SimpleState, key: Bytes) -> BytesWithError {
    let obj = unsafe { &mut *obj_ptr };
    let key = key.get_slice();
    let value = obj.get_value(&key.to_vec());

    match value {
        Some(v) => BytesWithError {
            data: v.as_ptr() as *mut u8,
            len: v.len(),
            error: std::ptr::null(),
        },
        None => {
            // this should error
            // could add an extra field to bytes to indicate error, or
            // update a pointer to an error message
            BytesWithError {
                data: std::ptr::null_mut(),
                len: 0,
                error: CString::new(ERR_NOT_FOUND).unwrap().into_raw(),
            }
        }
    }
}

// define constant error messages
// pub const ERR_NONE: &str = "None";
// TODO: don't like how they need to be exactly like the go error messages. maybe can set up errors in the .h file?
pub const ERR_NOT_FOUND: &str = "not found";

pub extern "C" fn insert_state_callback(
    obj_ptr: *mut SimpleState,
    key: Bytes,
    value: Bytes,
) -> *const c_char {
    let obj = unsafe { &mut *obj_ptr };
    let key = key.get_slice();
    let value = value.get_slice();
    obj.insert(key.to_vec(), value.to_vec());
    // should be error message
    std::ptr::null()
    // when is this freed?
    // CString::new(ERR_NONE).unwrap().into_raw()
}

pub extern "C" fn remove_state_callback(obj_ptr: *mut SimpleState, key: Bytes) -> *const c_char {
    println!("remove state!");
    let obj = unsafe { &mut *obj_ptr };
    let key = key.get_slice();
    obj.remove(key.to_vec());
    // should be error message
    std::ptr::null()
}

// could have one callback function that multiplexes to different functions
// or pass in multiple function pointers
pub type GetStateCallback =
    extern "C" fn(simObjectPtr: *mut SimpleState, key: Bytes) -> BytesWithError;
pub type InsertStateCallback =
    extern "C" fn(objectPtr: *mut SimpleState, key: Bytes, value: Bytes) -> *const c_char;
pub type RemoveStateCallback =
    extern "C" fn(objectPtr: *mut SimpleState, key: Bytes) -> *const c_char;
