//! The `state` module provides functions for interacting with persistent
//! storage exposed by the host.
use crate::errors::StateError;
use crate::memory::to_smart_ptr;
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
    let caller = to_smart_ptr(caller.id())?;
    let value = to_smart_ptr(&value_bytes)?;
    let key = to_smart_ptr(key)?;

    match unsafe { _put(caller, key, value) } {
        0 => Ok(()),
        _ => Err(StateError::Write),
    }
}

/// Gets the bytes associated with the key from the host.
pub(crate) unsafe fn get_bytes(caller: &Program, key: &Key) -> Result<i64, StateError> {
    // prepend length to key
    let caller = to_smart_ptr(caller.id())?;
    let key = to_smart_ptr(key)?;
    Ok(unsafe { _get(caller, key) })
}
