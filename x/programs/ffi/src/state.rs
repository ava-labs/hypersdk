use std::collections::HashMap;

use crate::types::Bytes;

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
}


#[repr(C)]
pub struct Mutable {
    pub obj: *mut SimpleState,
    pub get_state: GetStateCallback,
}

pub extern "C" fn get_state_callback(obj_ptr: *mut SimpleState, key: Bytes) -> Bytes {
    let obj = unsafe { &mut *obj_ptr };
    let key = key.get_slice();
    let value = obj.get_value(&key.to_vec());

    match value {
        Some(v) => {
            Bytes {
                data: v.as_ptr() as *mut u8,
                len: v.len(),
            }
        },
        None => {
            println!("ERRROR* Value not found");
            // this should error
            // could add an extra field to bytes to indicate error, or 
            // update a pointer to an error message
            Bytes {
                data: std::ptr::null_mut(),
                len: 0,
            }
        }
    }
}


// could have one callback function that multiplexes to different functions
// or pass in multiple function pointers
pub type GetStateCallback = extern fn(simObjectPtr: *mut SimpleState, key: Bytes) -> Bytes;
pub type InsertStateCallback = extern fn(*mut SimpleState) -> i32;
pub type RemoveStateCallback = extern fn(*mut SimpleState) -> i32;

