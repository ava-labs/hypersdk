use serde::{Serialize, de::DeserializeOwned};
use serde_bare::{to_vec, from_slice};

use crate::{errors::StateError, host::state::{put_bytes, len_bytes, get_bytes}, program::Program};

pub struct State {
    program: Program,
}

impl State {
    pub fn new(program: Program) -> Self {
        Self { program }
    }

    pub fn program_id(&self) -> Program {
        self.program
    }
     /// Insert a key and value to the host storage.
    ///
    /// # Errors Returns an `StateError` if the key or value cannot be
    /// serialized or if the host fails to write the key and value.
    pub fn insert<V>(&self, key: &[u8], value: &V) -> Result<(), StateError>
    where
        V: Serialize,
    {
        let value_bytes = to_vec(value).map_err(|_| StateError::Serialization)?;
        match unsafe {
            put_bytes(
                &self.program,
                key.as_ptr(),
                key.len(),
                value_bytes.as_ptr(),
                value_bytes.len(),
            )
        } {
            0 => Ok(()),
            _ => Err(StateError::Write),
        }
    }

       pub fn get_value<T>(&self, key: &[u8]) -> Result<T, StateError>
where
    T: DeserializeOwned,
{
    let key_ptr = key.as_ptr();
    let key_len = key.len();

    let val_len = unsafe { len_bytes(&self.program, key_ptr, key_len) };
    let val_ptr = unsafe { get_bytes(&self.program, key_ptr, key_len, val_len) };
    if val_ptr < 0 {
        return Err(StateError::Read);
    }

    let val =
        unsafe { Vec::from_raw_parts(val_ptr as *mut u8, val_len as usize, val_len as usize) };
    from_slice(&val).map_err(|_| StateError::InvalidBytes)
}

}