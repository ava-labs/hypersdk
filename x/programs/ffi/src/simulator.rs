use std::collections::HashMap;

#[repr(C)]
pub struct Simulator {
    state: HashMap<Vec<u8>, Vec<u8>>,
}

impl Simulator {
   pub fn new() -> Simulator {
        Simulator { 
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

pub extern "C" fn get_state_callback(obj_ptr: *mut Simulator, key_ptr: *const u8, key_len: usize) -> i32 {
    let obj = unsafe { &mut *obj_ptr };
    let key_bytes = unsafe { std::slice::from_raw_parts(key_ptr, key_len) };
    let value = obj.get_value(&key_bytes.to_vec());

    match value {
        Some(v) => {
            println!("Value: {:?}", v);
            0
        },
        None => {
            println!("Value not found");
            1
        }
    }
}


// could have one callback function that multiplexes to different functions
// or pass in multiple function pointers
pub type GetStateCallback = extern fn(*mut Simulator, *const u8, usize) -> i32;
pub type InsertStateCallback = extern fn(*mut Simulator) -> i32;
pub type RemoveStateCallback = extern fn(*mut Simulator) -> i32;

