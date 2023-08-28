use crate::errors::StorageError;
use crate::host::host_program_invoke;
use crate::host::{get_bytes, get_bytes_len, store_bytes};
use crate::types::Argument;
use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};
use serde_bare::{from_slice, to_vec};
use std::str;
/// Context defines helper methods for the program builder
/// to interact with the host.
#[derive(Clone, Copy, Serialize, Deserialize)]
pub struct Context {
    pub program_id: i64,
}

/// Fails if we are storing a map with non-string keys
impl Context {
    pub fn store_value<T>(&self, key: &str, value: &T) -> Result<(), StorageError>
    where
        T: Serialize,
    {
        let key_bytes = key.as_bytes();
        // Add the tag(u8) to the start of val_bytes

        store_key_value(self, key_bytes.to_vec(), value)
    }

    pub fn store_map_value<T, U>(
        &self,
        map_name: &str,
        key: &U,
        value: &T,
    ) -> Result<(), StorageError>
    where
        T: Serialize,
        U: Serialize,
    {
        let key_bytes = get_map_key(map_name, &key)?;
        // Add a tag(u8) to the start of val_bytes
        store_key_value(self, key_bytes, value)
    }
    pub fn get_value<T>(&self, name: &str) -> Result<T, StorageError>
    where
        T: DeserializeOwned,
    {
        get_field(self, name)
    }
    pub fn get_map_value<T, U>(&self, map_name: &str, key: &T) -> Result<U, StorageError>
    where
        T: Serialize,
        U: DeserializeOwned,
    {
        let result: U = get_map_field(self, map_name, key)?;
        Ok(result)
    }
}

impl From<Context> for i64 {
    fn from(ctx: Context) -> Self {
        ctx.program_id
    }
}

impl From<i64> for Context {
    fn from(value: i64) -> Self {
        Context { program_id: value }
    }
}

fn store_key_value<T>(ctx: &Context, key_bytes: Vec<u8>, value: &T) -> Result<(), StorageError>
where
    T: Serialize,
{
    let value_bytes = to_vec(value).map_err(|_| StorageError::SerializationError)?;
    match unsafe {
        store_bytes(
            ctx,
            key_bytes.as_ptr(),
            key_bytes.len(),
            value_bytes.as_ptr(),
            value_bytes.len(),
        )
    } {
        0 => Ok(()),
        _ => Err(StorageError::HostStoreError),
    }
}

fn get_field_as_bytes(ctx: &Context, name: &[u8]) -> Result<Vec<u8>, StorageError> {
    let name_ptr = name.as_ptr();
    let name_len = name.len();
    // First get the length of the bytes from the host.
    let bytes_len = unsafe { get_bytes_len(ctx, name_ptr, name_len) };
    // Speculation that compiler might be optimizing out this if statement.
    if bytes_len < 0 {
        return Err(StorageError::HostRetrieveError);
    }
    // Get_bytes allocates bytes_len memory in the WASM module.
    let bytes_ptr = unsafe { get_bytes(ctx, name_ptr, name_len, bytes_len) };
    // Defensive check here to unsure we don't grab out of bounds memory.
    if bytes_ptr < 0 {
        return Err(StorageError::HostRetrieveError);
    }
    let bytes_ptr = bytes_ptr as *mut u8;

    // Take ownership of those bytes grabbed from the host. We want Rust to manage the memory.
    let bytes = unsafe { Vec::from_raw_parts(bytes_ptr, bytes_len as usize, bytes_len as usize) };
    Ok(bytes)
}

/// Gets the field [name] from the host and returns it as a ProgramValue.
fn get_field<T>(ctx: &Context, name: &str) -> Result<T, StorageError>
where
    T: DeserializeOwned,
{
    let bytes = get_field_as_bytes(ctx, name.as_bytes())?;
    from_slice(&bytes).map_err(|_| StorageError::InvalidBytes)
}

/// Gets the correct key to in the host storage for a [map_name] and [key] within that map  
fn get_map_key<T>(map_name: &str, key: &T) -> Result<Vec<u8>, StorageError>
where
    T: Serialize,
{
    to_vec(key)
        .map(|bytes| [map_name.as_bytes(), &bytes].concat())
        .or(Err(StorageError::InvalidBytes))
}

// Gets the value from the map [name] with key [key] from the host and returns it as a ProgramValue.
fn get_map_field<T, U>(ctx: &Context, name: &str, key: &T) -> Result<U, StorageError>
where
    T: Serialize,
    U: DeserializeOwned,
{
    let map_key = get_map_key(name, key)?;
    let map_value = get_field_as_bytes(ctx, &map_key)?;
    from_slice(&map_value).map_err(|_| StorageError::HostRetrieveError)
}

/// Implement the program_invoke function for the Context which allows a program to
/// call another program.
impl Context {
    pub fn program_invoke(
        &self,
        call_ctx: &Context,
        fn_name: &str,
        call_args: &[Box<dyn Argument>],
    ) -> i64 {
        host_program_invoke(call_ctx, fn_name, &Self::marshal_args(call_args))
    }

    fn marshal_args(args: &[Box<dyn Argument>]) -> Vec<u8> {
        use std::mem::size_of;
        // Size of meta data for each argument
        const META_SIZE: usize = size_of::<i64>() + 1;

        // Calculate the total size of the combined byte slices
        let total_size = args.iter().map(|cow| cow.len() + META_SIZE).sum();

        // Create a mutable Vec<u8> to hold the combined bytes
        let mut bytes = Vec::with_capacity(total_size);

        for arg in args {
            // if we want to be efficient we dont need to add length of bytes if its an int
            let len = i64::try_from(arg.len()).expect("Error converting to i64");
            bytes.extend_from_slice(&len.as_bytes());
            bytes.extend_from_slice(&[arg.is_primitive() as u8]);
            bytes.extend_from_slice(&arg.as_bytes());
        }
        bytes
    }
}
