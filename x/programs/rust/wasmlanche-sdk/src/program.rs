use crate::{
    memory::HostPtr,
    state::{Key, State},
    types::Address,
    types::Id,
    Gas,
};
use borsh::{BorshDeserialize, BorshSerialize};
use std::{cell::RefCell, collections::HashMap, io::Read};
use thiserror::Error;

/// Defer deserialization from bytes
/// <div class="warning">It is possible that this type performs multiple allocations during deserialization. It should be used sparingly.</div>
#[cfg_attr(feature = "debug", derive(Debug))]
pub struct DeferDeserialize(Vec<u8>);

impl DeferDeserialize {
    /// # Errors
    /// Returns a [`std::io::Error`] if there was an issue deserializing the value
    pub fn deserialize<T: BorshDeserialize>(self) -> Result<T, std::io::Error> {
        let Self(bytes) = self;
        borsh::from_slice(&bytes)
    }
}

impl BorshDeserialize for DeferDeserialize {
    /// <div class="warning">This function does not provide any size-hint.</div>
    fn deserialize_reader<R: Read>(reader: &mut R) -> std::io::Result<Self> {
        let mut inner = Vec::new();
        reader.read_to_end(&mut inner)?;
        Ok(Self(inner))
    }
}

/// An error that is returned from call to public functions.
#[derive(Error, Debug, BorshSerialize, BorshDeserialize, PartialEq, Eq)]
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
}

/// Represents the current Program in the context of the caller, or an external
/// program that is being invoked.
#[cfg_attr(feature = "debug", derive(Debug))]
pub struct Program<K = ()> {
    account: Address,
    state_cache: RefCell<HashMap<K, Option<Vec<u8>>>>,
}

impl<K> BorshSerialize for Program<K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            account,
            state_cache: _,
        } = self;

        account.serialize(writer)
    }
}

impl<K> BorshDeserialize for Program<K> {
    fn deserialize_reader<R: std::io::prelude::Read>(reader: &mut R) -> std::io::Result<Self> {
        let account: Address = BorshDeserialize::deserialize_reader(reader)?;
        Ok(Self {
            account,
            state_cache: RefCell::default(),
        })
    }
}

impl<K> Program<K> {
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
    /// # use wasmlanche_sdk::{types::Address, Program};
    /// #
    /// # let program_id = [0; Address::LEN];
    /// # let target: Program<()> = borsh::from_slice(&program_id).unwrap();
    /// let increment = 10;
    /// let params = borsh::to_vec(&increment).unwrap();
    /// let max_units = 1000000;
    /// let has_incremented: bool = target.call_function("increment", &params, max_units)?;
    /// assert!(has_incremented);
    /// # Ok::<(), wasmlanche_sdk::ExternalCallError>(())
    /// ```
    pub fn call_function<T: BorshDeserialize>(
        &self,
        function_name: &str,
        args: &[u8],
        max_units: Gas,
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
        };

        let args_bytes = borsh::to_vec(&args).expect("failed to serialize args");

        let bytes = unsafe { call_program(args_bytes.as_ptr(), args_bytes.len()) };

        borsh::from_slice(&bytes).expect("failed to deserialize")
    }

    /// Gets the remaining fuel available to this program
    /// # Panics
    /// Panics if there was an issue deserializing the remaining fuel
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

impl<K: Key> Program<K> {
    /// Returns a State object that can be used to interact with persistent
    /// storage exposed by the host.
    #[must_use]
    pub fn state(&self) -> State<K> {
        State::new(&self.state_cache)
    }
}

struct CallProgramArgs<'a, K> {
    target: &'a Program<K>,
    function: &'a [u8],
    args: &'a [u8],
    max_units: Gas,
}

impl<K> BorshSerialize for CallProgramArgs<'_, K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            target,
            function,
            args,
            max_units,
        } = self;

        target.serialize(writer)?;
        function.serialize(writer)?;
        args.serialize(writer)?;
        max_units.serialize(writer)?;

        Ok(())
    }
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
