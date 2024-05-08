use borsh::BorshSerialize;

use crate::state::Error;

/// Persists the bytes at key on the host storage.
pub(super) fn put_bytes(bytes: &[u8]) -> Result<(), Error> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "put"]
        fn ffi(ptr: *const u8, len: usize) -> usize;
    }

    let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };

    match result {
        0 => Ok(()),
        _ => Err(Error::Write),
    }
}

/// Gets the bytes associated with the key from the host.
pub(super) fn get_bytes(bytes: &[u8]) -> Result<*const u8, Error> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "get"]
        fn ffi(ptr: *const u8, len: usize) -> *const u8;
    }

    let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };

    if result.is_null() {
        Err(Error::Read)
    } else {
        Ok(result)
    }
}

/// Deletes the bytes at key ptr from the host storage
pub(super) fn delete_bytes(bytes: &[u8]) -> Result<(), Error> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "delete"]
        fn ffi(ptr: *const u8, len: usize) -> i32;
    }

    let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };
    match result {
        0 => Ok(()),
        _ => Err(Error::Delete),
    }
}

/// Logging facility for debugging purposes
pub fn log_bytes(bytes: &[u8]) -> Result<(), Error> {
    #[link(wasm_import_module = "state")]
    extern "C" {
        #[link_name = "log"]
        fn ffi(ptr: *const u8, len: usize) -> i32;
    }

    let result = unsafe { ffi(bytes.as_ptr(), bytes.len()) };
    match result {
        0 => Ok(()),
        _ => Err(Error::Log),
    }
}

pub fn log<V>(val: V) -> Result<(), Error>
where
    V: BorshSerialize,
{
    // let val_bytes = borsh::to_vec(&val).map_err(|_| StateError::Serialization)?;
    let val_bytes = borsh::to_vec(&val).unwrap();

    log_bytes(&val_bytes)
}
