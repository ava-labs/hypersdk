use std::collections::HashMap;

#[repr(C)]
pub struct Simulator {
    pub state: HashMap<Vec<u8>, Vec<u8>>,
}

impl Simulator {
   pub fn new() -> Simulator {
        Simulator { 
            state: HashMap::new(),
         }
    }
}

pub extern "C" fn get_state_callback(obj_ptr: *mut Simulator) -> i32 {
    let obj = unsafe { &mut *obj_ptr };
    println!("Im called from C with value: {}", 10090);
    888
}


// could have one callback function that multiplexes to different functions
// or pass in multiple function pointers
pub type GetStateCallback = extern fn(*mut Simulator) -> i32;
pub type InsertStateCallback = extern fn(*mut Simulator) -> i32;
pub type RemoveStateCallback = extern fn(*mut Simulator) -> i32;

