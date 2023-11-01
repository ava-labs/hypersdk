//! The `state` module provides functions for interacting with persistent
//! storage exposed by the host.
use crate::{
    errors::StateError,
    program::Program,
    state::{Key, Storable, Value},
};

#[link(wasm_import_module = "state")]
extern "C" {
    #[link_name = "put"]
    fn _put(caller_id: i64, storable_key: i64, storable_value: i64) -> i64;

    #[link_name = "get"]
    fn _get(caller_id: i64, storable_key: i64) -> i64;

    #[link_name = "delete"]
    fn _delete(caller_id: i64, storable_key: i64) -> i64;
}

/// Persists `Storable` object to host storage.
#[must_use]
pub(crate) fn put_bytes<const M: usize, const N: usize>(
    caller: &Program,
    storable: Storable<M, N>,
) -> Result<(), StateError> {
    let resp = unsafe { _put(caller.id(), storable.key().into(), storable.value().into()) };
    if resp.is_negative() {
        return Err(StateError::Write);
    }

    Ok(())
}

/// Returns a `Value` associated with the `Key` from the host.
#[must_use]
pub(crate) fn get_bytes<const M: usize, const N: usize>(
    caller: &Program,
    storable_key: Key<M>,
) -> Result<Value<N>, StateError> {
    let resp = unsafe { _get(caller.id(), storable_key.into()) };
    if resp.is_negative() {
        return Err(StateError::Read);
    }

    Ok(resp.into())
}

/// Deletes the bytes associated with the `Key` from the host.
#[must_use]
pub(crate) fn delete_bytes<const M: usize>(
    caller: &Program,
    storable_key: Key<M>,
) -> Result<(), StateError> {
    let resp = unsafe { _delete(caller.id(), storable_key.into()) };
    if resp.is_negative() {
        return Err(StateError::Delete);
    }

    Ok(())
}
