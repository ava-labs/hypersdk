
// #[repr(C)]
// pub struct Mutable {
//     // stores three closures that implement 
//     pub get_value: extern "C" fn(key: *const c_uchar, key_length: c_uint) -> Response,
//     pub insert: extern "C" fn(key: *const c_uchar, key_length: c_uint, value: *const c_uchar, value_length: c_uint) -> Response,
//     pub delete: extern "C" fn(key: *const c_uchar, key_length: c_uint) -> Response,
// }

// // to implement state.Mutable we need to implement the following functions
// // - GetValue(key []byte) ([]byte, error)
// // - Insert(key []byte, value []byte) error
// // - Delete(key []byte) error

// pub struct SimpleState {
//     hashmap: std::collections::HashMap<Vec<u8>, Vec<u8>>,
// }

// impl SimpleState {
//     fn new() -> Self {
//         SimpleState {
//             hashmap: std::collections::HashMap::new(),
//         }
//     }

//     fn get_value(&self, key: &[u8]) -> Result<Vec<u8>, String> {
//         match self.hashmap.get(key) {
//             Some(value) => Ok(value.clone()),
//             None => Err("Key not found".to_string()),
//         }
//     }

//     fn insert(&mut self, key: Vec<u8>, value: Vec<u8>) -> Result<(), String> {
//         self.hashmap.insert(key, value);
//         Ok(())
//     }

//     fn delete(&mut self, key: Vec<u8>) -> Result<(), String> {
//         self.hashmap.remove(&key);
//         Ok(())
//     }


//     // wrap in extern "C" functions
//     extern "C" fn _get_value(&self, key: *const c_uchar, key_length: c_uint) -> Response {
//         // let key = unsafe { std::slice::from_raw_parts(key, key_length as usize) };
//         // match self.get_value(key) {
//         //     Ok(value) => Response {
//         //         id: 0,
//         //         error: std::ptr::null(),
//         //         result: value.as_ptr(),
//         //     },
//         //     Err(e) => Response {
//         //         id: 1,
//         //         error: CString::new(e).unwrap().as_ptr(),
//         //         result: std::ptr::null(),
//         //     },
//         // }
//         println!("get value called with key_length: {}", key_length);
//         Response {
//             id: 1000,
//             error: std::ptr::null(),
//             result: std::ptr::null(),
//         }
//     }

//     extern "C" fn _insert(&self, key: *const c_uchar, key_length: c_uint, value: *const c_uchar, value_length: c_uint) -> Response {
//         // let key = unsafe { std::slice::from_raw_parts(key, key_length as usize) };
//         // let value = unsafe { std::slice::from_raw_parts(value, value_length as usize) };
//         // match self.insert(key.to_vec(), value.to_vec()) {
//         //     Ok(_) => Response {
//         //         id: 0,
//         //         error: std::ptr::null(),
//         //         result: std::ptr::null(),
//         //     },
//         //     Err(e) => Response {
//         //         id: 1,
//         //         error: CString::new(e).unwrap().as_ptr(),
//         //         result: std::ptr::null(),
//         //     },
//         // }
//         println!("insert value called with key_length: {}", key_length);
//         Response {
//             id: 1000,
//             error: std::ptr::null(),
//             result: std::ptr::null(),
//         }
//     }

//     extern "C" fn _delete(&self, key: *const c_uchar, key_length: c_uint) -> Response {
//         // let key = unsafe { std::slice::from_raw_parts(key, key_length as usize) };
//         // match self.delete(key.to_vec()) {
//         //     Ok(_) => Response {
//         //         id: 0,
//         //         error: std::ptr::null(),
//         //         result: std::ptr::null(),
//         //     },
//         //     Err(e) => Response {
//         //         id: 1,
//         //         error: CString::new(e).unwrap().as_ptr(),
//         //         result: std::ptr::null(),
//         //     },
//         // }
//         println!("delete called with key_length: {}", key_length);
//         Response {
//             id: 1000,
//             error: std::ptr::null(),
//             result: std::ptr::null(),
//         }
//     }
// }


