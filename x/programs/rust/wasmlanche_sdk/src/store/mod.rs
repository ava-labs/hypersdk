use crate::errors::StorageError;
use crate::host::{get_bytes, get_bytes_len, invoke_program, store_bytes};
use crate::types::Argument;
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_bare::{from_slice, to_vec};
use std::str;

#[allow(clippy::module_name_repetitions)]
pub enum StoreResult {
    Ok(State),
    Err(StorageError),
}

impl StoreResult {
    #[must_use]
    pub fn store_value<T>(self, key: &str, value: &T) -> Self
    where
        T: Serialize + ?Sized,
    {
        match self {
            Self::Ok(state) => state.store_value(key, value),
            err @ Self::Err(_) => err,
        }
    }

    #[must_use]
    pub fn store_map_value<T, U>(self, map_name: &str, key: &U, value: &T) -> Self
    where
        T: Serialize + ?Sized,
        U: Serialize,
    {
        match self {
            Self::Ok(state) => state.store_map_value(map_name, key, value),
            err @ Self::Err(_) => err,
        }
    }

    #[must_use]
    pub fn is_ok(self) -> bool {
        matches!(self, Self::Ok(_))
    }

    #[must_use]
    pub fn is_err(self) -> bool {
        matches!(self, Self::Err(_))
    }
}

/// State defines helper methods for the program builder
/// to interact with the host.
#[derive(Clone, Copy, Serialize, Deserialize)]
pub struct State {
    pub program_id: i64,
}

/// Fails if we are storing a map with non-string keys
impl State {
    pub fn store_value<T>(self, key: &str, value: &T) -> StoreResult
    where
        T: Serialize + ?Sized,
    {
        let key_bytes = key.as_bytes();

        match store_key_value(self, key_bytes, value) {
            Ok(()) => StoreResult::Ok(self),
            Err(e) => StoreResult::Err(e),
        }
    }

    pub fn store_map_value<T, U>(self, map_name: &str, key: &U, value: &T) -> StoreResult
    where
        T: Serialize + ?Sized,
        U: Serialize,
    {
        let key_bytes = match get_map_key(map_name, &key) {
            Ok(bytes) => bytes,
            Err(e) => return StoreResult::Err(e),
        };

        match store_key_value(self, &key_bytes, value) {
            Ok(()) => StoreResult::Ok(self),
            Err(e) => StoreResult::Err(e),
        }
    }

    /// Returns the value of the field `name` from the host.
    /// # Errors
    /// Returns a `StorageError` if there was an error retrieving from the host or the retrieved bytes are invalid.
    pub fn get_value<T>(self, name: &str) -> Result<T, StorageError>
    where
        T: DeserializeOwned,
    {
        get_field(self, name)
    }

    /// Returns the value of the field `key` from the map `map_name` from the host.
    /// # Errors
    /// Returns a `StorageError` if there was an error retrieving from the host or the retrieved bytes are invalid.
    pub fn get_map_value<T, U>(self, map_name: &str, key: &T) -> Result<U, StorageError>
    where
        T: Serialize,
        U: DeserializeOwned,
    {
        let result: U = get_map_field(self, map_name, key)?;
        Ok(result)
    }
}

impl From<State> for i64 {
    fn from(state: State) -> Self {
        state.program_id
    }
}

impl From<i64> for State {
    fn from(value: i64) -> Self {
        State { program_id: value }
    }
}

fn store_key_value<T>(state: State, key_bytes: &[u8], value: &T) -> Result<(), StorageError>
where
    T: Serialize + ?Sized,
{
    let value_bytes = to_vec(value).map_err(|_| StorageError::SerializationError)?;
    match unsafe {
        store_bytes(
            state,
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

fn get_field_as_bytes(state: State, name: &[u8]) -> Result<Vec<u8>, StorageError> {
    let name_ptr = name.as_ptr();
    let name_len = name.len();
    // First get the length of the bytes from the host.
    let bytes_len = unsafe { get_bytes_len(state, name_ptr, name_len) };
    // Speculation that compiler might be optimizing out this if statement.
    if bytes_len < 0 {
        return Err(StorageError::HostRetrieveError);
    }
    // Get_bytes allocates bytes_len memory in the WASM module.
    let bytes_ptr = unsafe { get_bytes(state, name_ptr, name_len, bytes_len) };
    // Defensive check here to unsure we don't grab out of bounds memory.
    if bytes_ptr < 0 {
        return Err(StorageError::HostRetrieveError);
    }
    let bytes_ptr = bytes_ptr as *mut u8;

    // Take ownership of those bytes grabbed from the host. We want Rust to manage the memory.
    let bytes = unsafe {
        Vec::from_raw_parts(
            bytes_ptr,
            bytes_len.try_into().unwrap(),
            bytes_len.try_into().unwrap(),
        )
    };
    Ok(bytes)
}

/// Gets the field `name` from the host and returns it as a `ProgramValue`.
fn get_field<T>(state: State, name: &str) -> Result<T, StorageError>
where
    T: DeserializeOwned,
{
    let bytes = get_field_as_bytes(state, name.as_bytes())?;
    from_slice(&bytes).map_err(|_| StorageError::InvalidBytes)
}

/// Gets the correct key to in the host storage for a `map_name` and `key` within that map
fn get_map_key<T>(map_name: &str, key: &T) -> Result<Vec<u8>, StorageError>
where
    T: Serialize,
{
    to_vec(key)
        .map(|bytes| [map_name.as_bytes(), &bytes].concat())
        .or(Err(StorageError::InvalidBytes))
}

// Gets the value from the map [name] with key [key] from the host and returns it as a ProgramValue.
fn get_map_field<T, U>(state: State, name: &str, key: &T) -> Result<U, StorageError>
where
    T: Serialize,
    U: DeserializeOwned,
{
    let map_key = get_map_key(name, key)?;
    let map_value = get_field_as_bytes(state, &map_key)?;
    from_slice(&map_value).map_err(|_| StorageError::HostRetrieveError)
}

/// Implement the `invoke_program` function for the State which allows a program to
/// call another program.
impl State {
    #[must_use]
    pub fn invoke_program(
        &self,
        call_state: State,
        fn_name: &str,
        call_args: &[Box<dyn Argument>],
    ) -> i64 {
        invoke_program(call_state, fn_name, &Self::marshal_args(call_args))
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
            bytes.extend_from_slice(&[u8::from(arg.is_primitive())]);
            bytes.extend_from_slice(&arg.as_bytes());
        }
        bytes
    }
}
