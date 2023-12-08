//! The `state` module provides functions for interacting with persistent
//! storage exposed by the host.
use crate::errors::StateError;
use crate::memory::to_ptr_arg;
use crate::{program::Program, state::Key};
use borsh::{to_vec, BorshSerialize};

#[link(wasm_import_module = "state")]
extern "C" {
    #[link_name = "put"]
    fn _put(caller: i64, key: i64, value: i64) -> i32;

    #[link_name = "get"]
    fn _get(caller: i64, key: i64) -> i64;
}

/// Persists the bytes at `value_ptr` to the bytes at key ptr on the host storage.
pub(crate) unsafe fn put_bytes<V>(caller: &Program, key: &Key, value: &V) -> Result<(), StateError>
where
    V: BorshSerialize,
{
    let value_bytes = to_vec(value).map_err(|_| StateError::Serialization)?;
    // prepend length to both key & value
    let caller_id = caller.id();
    let caller = to_ptr_arg(&caller_id)?;
    let value = to_ptr_arg(&value_bytes)?;
    let key = to_ptr_arg(key)?;

    match unsafe { _put(caller, key, value) } {
        0 => Ok(()),
        _ => Err(StateError::Write),
    }
}

/// Gets the bytes associated with the key from the host.
pub(crate) unsafe fn get_bytes(caller: &Program, key: &Key) -> i64 {
    // prepend length to key
    let caller_id = caller.id();
    let caller = to_ptr_arg(&caller_id).unwrap();
    let key = to_ptr_arg(key).unwrap();
    unsafe { _get(caller, key) }
}
