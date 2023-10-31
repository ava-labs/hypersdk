//! The `state` module provides functions for interacting with persistent
//! storage exposed by the host.
use crate::{
    errors::StateError,
    memory::Pointer,
    program::Program,
    state::{Key, Storable},
};

#[link(wasm_import_module = "state")]
extern "C" {
    #[link_name = "put"]
    fn _put(caller_id: i64, storable_key: i64, storable_value: i64) -> i32;

    #[link_name = "get"]
    fn _get(caller_id: i64, storable_key: i64, val_len: i32) -> i32;

    #[link_name = "len"]
    fn _len(caller_id: i64, storable_key: i64) -> i32;
}

/// Persists the bytes at `value_ptr` to the bytes at key ptr on the host storage.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` and
/// `value_ptr` + `value_len` point to valid memory locations.
#[must_use]
pub(crate) unsafe fn put_bytes<const M: usize, const N: usize>(
    caller: &Program,
    storable: &Storable<M, N>,
) -> i32 {
    unsafe { _put(caller.id(), storable.key(), storable.value()) }
}

/// Returns the length of the bytes associated with the key from the host storage.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` points to valid memory locations.
#[must_use]
pub(crate) unsafe fn len_bytes<const M: usize>(
    caller: &Program,
    storable_key: &Key<M>,
) -> Result<i32, StateError> {
    let resp = unsafe { _len(caller.id(), storable_key) };
    if resp.is_negative() {
        return Err(StateError::Read);
    }
    Ok(resp)
}

/// Gets the bytes associated with the key from the host.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` points to valid memory locations.
#[must_use]
pub(crate) unsafe fn get_bytes<const M: usize>(
    caller: &Program,
    storable_key: &Key<M>,
    val_len: i32,
) -> Result<Pointer, StateError> {
    let resp = unsafe { _get(caller.id(), storable_key, val_len) };
    if resp.is_negative() {
        return Err(StateError::Read);
    }
    Ok(resp.into())
}
