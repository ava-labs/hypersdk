//! The `state` module provides functions for interacting with persistent
//! storage exposed by the host.
use crate::program::Program;

#[link(wasm_import_module = "state")]
extern "C" {
    #[link_name = "put"]
    fn _put(
        caller_id: i64,
        key_ptr: *const u8,
        key_len: usize,
        value_ptr: *const u8,
        value_len: usize,
    ) -> i32;

    #[link_name = "get"]
    fn _get(caller_id: i64, key_ptr: *const u8, key_len: usize, val_len: i32) -> i32;

    #[link_name = "len"]
    fn _len(caller_id: i64, key_ptr: *const u8, key_len: usize) -> i32;
}

/// Persists the bytes at `value_ptr` to the bytes at key ptr on the host storage.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` and
/// `value_ptr` + `value_len` point to valid memory locations.
#[must_use]
pub(crate) unsafe fn put_bytes(
    caller: &Program,
    key_ptr: *const u8,
    key_len: usize,
    value_ptr: *const u8,
    value_len: usize,
) -> i32 {
    unsafe { _put(caller.id(), key_ptr, key_len, value_ptr, value_len) }
}

/// Returns the length of the bytes associated with the key from the host storage.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` points to valid memory locations.
#[must_use]
pub(crate) unsafe fn len_bytes(caller: &Program, key_ptr: *const u8, key_len: usize) -> i32 {
    unsafe { _len(caller.id(), key_ptr, key_len) }
}

/// Gets the bytes associated with the key from the host.
///
/// # Safety
/// The caller must ensure that `key_ptr` + `key_len` points to valid memory locations.
#[must_use]
pub(crate) unsafe fn get_bytes(
    caller: &Program,
    key_ptr: *const u8,
    key_len: usize,
    val_len: i32,
) -> i32 {
    unsafe { _get(caller.id(), key_ptr, key_len, val_len) }
}
