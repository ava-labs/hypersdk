use crate::{
    memory::HostPtr,
    state::{Error as StateError, Key, State},
    types::Address,
    Gas,
};
use borsh::{BorshDeserialize, BorshSerialize};
use std::{cell::RefCell, collections::HashMap};

/// Represents the current Program in the context of the caller. Or an external
/// program that is being invoked.
#[derive(Clone, Debug)]
pub struct Program<K = ()> {
    account: Address,
    state_cache: RefCell<HashMap<K, Vec<u8>>>,
}

impl<K> BorshSerialize for Program<K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            account,
            state_cache: _,
        } = self;
        BorshSerialize::serialize(account, writer)
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
    /// Returns a [`StateError`] if the call fails.
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_function<T: BorshDeserialize, ArgType: BorshSerialize>(
        &self,
        function_name: &str,
        args: ArgType,
        max_units: Gas,
    ) -> Result<T, StateError> {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "call_program"]
            fn call_program(ptr: *const u8, len: usize) -> HostPtr;
        }

        let args_ptr = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

        let args = CallProgramArgs {
            target: self,
            function: function_name.as_bytes(),
            args_ptr: &args_ptr,
            max_units,
        };

        let args_bytes = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

        let bytes = unsafe { call_program(args_bytes.as_ptr(), args_bytes.len()) };

        borsh::from_slice(&bytes).map_err(|_| StateError::Deserialization)
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
    args_ptr: &'a [u8],
    max_units: Gas,
}

impl<K> BorshSerialize for CallProgramArgs<'_, K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            target,
            function,
            args_ptr,
            max_units,
        } = self;
        BorshSerialize::serialize(target, writer)?;
        BorshSerialize::serialize(function, writer)?;
        BorshSerialize::serialize(args_ptr, writer)?;
        BorshSerialize::serialize(max_units, writer)?;
        Ok(())
    }
}
