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
    fn _put(caller_id: i64, key_ptr: *const u8, value: *const u8) -> i32;

    #[link_name = "get"]
    fn _get(caller_id: i64, key_ptr: *const u8) -> i64;
}

/// Persists the bytes at `value_ptr` to the bytes at key ptr on the host storage.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` and
/// `value_ptr` + `value_len` point to valid memory locations.
#[must_use]
pub(crate) unsafe fn put_bytes<V>(caller: &Program, key: &Key, value: &V) -> Result<(), StateError>
where
    V: BorshSerialize,
{
    let value_bytes = to_vec(value).map_err(|_| StateError::Serialization)?;

    // prepend length to both key & value
    let value_bytes = prepend_length(&value_bytes);
    let key_bytes = prepend_length(key.as_bytes());

    match unsafe { _put(caller.id(), key_bytes.as_ptr(), value_bytes.as_ptr()) } {
        0 => Ok(()),
        _ => Err(StateError::Write),
    }
}

/// Returns the length of the bytes associated with the key from the host storage.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` points to valid memory locations.
// #[must_use]
// pub(crate) unsafe fn len_bytes(caller: &Program, key: &Key) -> i32 {
//     unsafe { _len(caller.id(), key.as_bytes().as_ptr(), key.len()) }
// }

/// Gets the bytes associated with the key from the host.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` points to valid memory locations.
#[must_use]
pub(crate) unsafe fn get_bytes(caller: &Program, key: &Key) -> i64 {
    // prepend length to key
    let key = prepend_length(key.as_bytes());
    unsafe { _get(caller.id(), key.as_ptr()) }
}
