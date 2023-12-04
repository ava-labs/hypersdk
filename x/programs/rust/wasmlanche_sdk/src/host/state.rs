//! The `state` module provides functions for interacting with persistent
//! storage exposed by the host.
use crate::errors::StateError;
use crate::{
    program::Program,
    state::{prepend_length, Key},
};
use borsh::{to_vec, BorshSerialize};

#[link(wasm_import_module = "state")]
extern "C" {
    #[link_name = "put"]
    fn _put(caller_id: *const u8, key_ptr: *const u8, value: *const u8) -> i32;

    #[link_name = "get"]
    fn _get(caller_id: *const u8, key_ptr: *const u8) -> i64;
}

/// Persists the bytes at `value_ptr` to the bytes at key ptr on the host storage.
///
/// # Safety
/// TODO:
pub(crate) unsafe fn put_bytes<V>(caller: &Program, key: &Key, value: &V) -> Result<(), StateError>
where
    V: BorshSerialize,
{
    let value_bytes = to_vec(value).map_err(|_| StateError::Serialization)?;

    // prepend length to both key & value
    let value_bytes = prepend_length(&value_bytes);
    let key_bytes = prepend_length(key.as_bytes());

    match unsafe { _put(caller.id().as_ptr(), key_bytes.as_ptr(), value_bytes.as_ptr()) } {
        0 => Ok(()),
        _ => Err(StateError::Write),
    }
}

/// Gets the bytes associated with the key from the host.
///
/// # Safety
/// TODO:
pub(crate) unsafe fn get_bytes(caller: &Program, key: &Key) -> i64 {
    // prepend length to key
    let key = prepend_length(key.as_bytes());
    unsafe { _get(caller.id().as_ptr(), key.as_ptr()) }
}
