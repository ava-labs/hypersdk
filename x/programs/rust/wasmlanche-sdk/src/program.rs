use crate::{memory::HostPtr, types::Address, types::Id, Gas};
use borsh::{BorshDeserialize, BorshSerialize};
use std::io::Read;
use thiserror::Error;

/// Defer deserialization from bytes
/// <div class="warning">It is possible that this type performs multiple allocations during deserialization. It should be used sparingly.</div>
#[cfg_attr(feature = "debug", derive(Debug))]
pub struct DeferDeserialize(Vec<u8>);

impl BorshSerialize for DeferDeserialize {
    /// # Errors
    /// Returns a [`std::io::Error`] if there was an issue writing
    fn serialize<W: std::io::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        writer.write_all(&self.0)
    }
}

impl DeferDeserialize {
    /// # Errors
    /// Returns a [`std::io::Error`] if there was an issue deserializing the value
    pub fn deserialize<T: BorshDeserialize>(self) -> Result<T, std::io::Error> {
        let Self(bytes) = self;
        borsh::from_slice(&bytes)
    }
}

impl BorshDeserialize for DeferDeserialize {
    fn deserialize_reader<R: Read>(reader: &mut R) -> std::io::Result<Self> {
        let mut inner = Vec::new();
        reader.read_to_end(&mut inner)?;
        Ok(Self(inner))
    }
}

#[allow(clippy::from_over_into)]
impl Into<Vec<u8>> for DeferDeserialize {
    fn into(self) -> Vec<u8> {
        self.0
    }
}

/// An error that is returned from call to public functions.
#[derive(Error, Debug, BorshSerialize, BorshDeserialize)]
#[repr(u8)]
#[non_exhaustive]
#[borsh(use_discriminant = true)]
pub enum ExternalCallError {
    #[error("an error happened during the execution")]
    ExecutionFailure = 0,
    #[error("the call panicked")]
    CallPanicked = 1,
    #[error("not enough fuel to cover the execution")]
    OutOfFuel = 2,
    #[error("insufficient funds")]
    InsufficientFunds = 3,
}

/// Transfer currency from the calling program to the passed address
/// # Panics
/// Panics if there was an issue deserializing the result
/// # Errors
/// Errors if there are insufficient funds
pub fn send(to: Address, amount: u64) -> Result<(), ExternalCallError> {
    #[link(wasm_import_module = "balance")]
    extern "C" {
        #[link_name = "send"]
        fn send_value(ptr: *const u8, len: usize) -> HostPtr;
    }
    let ptr = borsh::to_vec(&(to, amount)).expect("failed to serialize args");

    let bytes = unsafe { send_value(ptr.as_ptr(), ptr.len()) };

    borsh::from_slice(&bytes).expect("failed to deserialize the result")
}

/// Represents the current Program in the context of the caller, or an external
/// program that is being invoked.
#[cfg_attr(feature = "debug", derive(Debug))]
#[derive(Clone, Copy, BorshSerialize, BorshDeserialize)]
pub struct Program {
    account: Address,
}

impl Program {
    #[must_use]
    pub fn account(&self) -> &Address {
        &self.account
    }

    /// Attempts to call a function `name` with `args` on the given program. This method
    /// is used to call functions on external programs.
    /// # Errors
    /// Returns a [`ExternalCallError`] if the call fails.
    /// # Panics
    /// Will panic if the args cannot be serialized
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    /// # Examples
    /// ```no_run
    /// # use wasmlanche_sdk::{Address, Program};
    /// #
    /// # let program_id = [0; Address::LEN];
    /// # let target: Program = borsh::from_slice(&program_id).unwrap();
    /// let increment = 10;
    /// let params = borsh::to_vec(&increment).unwrap();
    /// let max_units = 1000000;
    /// let value = 0;
    /// let has_incremented: bool = target.call_function("increment", &params, max_units, value)?;
    /// assert!(has_incremented);
    /// # Ok::<(), wasmlanche_sdk::ExternalCallError>(())
    /// ```
    pub fn call_function<T: BorshDeserialize>(
        &self,
        function_name: &str,
        args: &[u8],
        max_units: Gas,
        max_value: u64,
    ) -> Result<T, ExternalCallError> {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "call_program"]
            fn call_program(ptr: *const u8, len: usize) -> HostPtr;
        }

        let args = CallProgramArgs {
            target: self,
            function: function_name.as_bytes(),
            args,
            max_units,
            max_value,
        };

        let args_bytes = borsh::to_vec(&args).expect("failed to serialize args");

        let bytes = unsafe { call_program(args_bytes.as_ptr(), args_bytes.len()) };

        borsh::from_slice(&bytes).expect("failed to deserialize")
    }

    /// Gets the remaining fuel available to this program
    /// # Panics
    /// Panics if there was an issue deserializing the remaining fuel
    #[must_use]
    pub fn remaining_fuel(&self) -> u64 {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "remaining_fuel"]
            fn get_remaining_fuel() -> HostPtr;
        }

        let bytes = unsafe { get_remaining_fuel() };

        borsh::from_slice::<u64>(&bytes).expect("failed to deserialize the remaining fuel")
    }

    /// Deploy an instance of the specified program and returns the account of the new instance
    /// # Panics
    /// Panics if there was an issue deserializing the account
    #[must_use]
    pub fn deploy(&self, program_id: Id, account_creation_data: &[u8]) -> Address {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "deploy"]
            fn deploy(ptr: *const u8, len: usize) -> HostPtr;
        }
        let ptr =
            borsh::to_vec(&(program_id, account_creation_data)).expect("failed to serialize args");

        let bytes = unsafe { deploy(ptr.as_ptr(), ptr.len()) };

        borsh::from_slice(&bytes).expect("failed to deserialize the account")
    }
}

#[derive(BorshSerialize)]
struct CallProgramArgs<'a> {
    target: &'a Program,
    function: &'a [u8],
    args: &'a [u8],
    max_units: Gas,
    max_value: u64,
}

#[cfg(test)]
mod tests {
    use super::DeferDeserialize;

    #[test]
    fn defer_bytes() {
        type ExpectedType = u64;

        let expected: ExpectedType = 42;
        let serialized = borsh::to_vec(&expected).unwrap();
        let deferred: DeferDeserialize = borsh::from_slice(&serialized).unwrap();
        assert_eq!(deferred.0, serialized);
        let actual = deferred.deserialize::<ExpectedType>().unwrap();
        assert_eq!(actual, expected);
    }
}
