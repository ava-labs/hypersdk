use crate::{
    memory::HostPtr,
    state::{Error as StateError, Key, State},
    Gas,
};
use borsh::{BorshDeserialize, BorshSerialize};
use std::{cell::RefCell, collections::HashMap};

pub const PROGRAM_ID_LEN: usize = 32;
type Id = [u8; PROGRAM_ID_LEN];

/// Represents the current Program in the context of the caller. Or an external
/// program that is being invoked.
#[derive(Clone, Debug)]
pub struct Program<K = ()> {
    id: Id,
    state_cache: RefCell<HashMap<K, Vec<u8>>>,
}

impl<K> BorshSerialize for Program<K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self { id, state_cache: _ } = self;
        BorshSerialize::serialize(id, writer)
    }
}

impl<K> BorshDeserialize for Program<K> {
    fn deserialize_reader<R: std::io::prelude::Read>(reader: &mut R) -> std::io::Result<Self> {
        let id: Id = BorshDeserialize::deserialize_reader(reader)?;
        Ok(Self {
            id,
            state_cache: RefCell::default(),
        })
    }
}

impl<K> Program<K> {
    #[must_use]
    pub fn id(&self) -> &[u8; PROGRAM_ID_LEN] {
        &self.id
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
            target_id: self,
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
    target_id: &'a Program<K>,
    function: &'a [u8],
    args_ptr: &'a [u8],
    max_units: Gas,
}

impl<K> BorshSerialize for CallProgramArgs<'_, K> {
    fn serialize<W: std::io::prelude::Write>(&self, writer: &mut W) -> std::io::Result<()> {
        let Self {
            target_id,
            function,
            args_ptr,
            max_units,
        } = self;
        BorshSerialize::serialize(target_id, writer)?;
        BorshSerialize::serialize(function, writer)?;
        BorshSerialize::serialize(args_ptr, writer)?;
        BorshSerialize::serialize(max_units, writer)?;
        Ok(())
    }
}
