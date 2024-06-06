use std::alloc::Layout;

use wasmlanche_sdk::{dbg, public, Context};

#[public]
pub fn get_value(_: Context) -> usize {
    let layout = Layout::array::<u8>(8).expect("capacity overflow");

    let ptr = unsafe { std::alloc::alloc(layout) };

    if ptr.is_null() {
        std::alloc::handle_alloc_error(layout);
    }

    let ptr =  ptr.cast_const() as usize;
    dbg!(ptr);
    ptr
} 

#[public]
pub fn get_ptr(_: Context, ptr: usize) -> i32 {
    unsafe {std::ptr::read(ptr as *const i32)}
}