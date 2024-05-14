use crate::{
    memory::into_bytes,
    state::{Error as StateError, Key, State},
    Params,
};
use borsh::{BorshDeserialize, BorshSerialize};
use std::hash::Hash;

/// Represents the current Program in the context of the caller. Or an external
/// program that is being invoked.
#[derive(Clone, Copy, BorshDeserialize, BorshSerialize, Debug)]
pub struct Program([u8; Self::LEN]);

impl Program {
    /// The length of ids.ID
    pub const LEN: usize = 32;

    /// Returns the id of the program.
    #[must_use]
    pub fn id(&self) -> &[u8; Self::LEN] {
        &self.0
    }

    /// Returns a State object that can be used to interact with persistent
    /// storage exposed by the host.
    #[must_use]
    pub fn state<K>(&self) -> State<K>
    where
        K: Into<Key> + Hash + PartialEq + Eq + Clone,
    {
        State::new()
    }

    /// Attempts to call a function `name` with `args` on the given program. This method
    /// is used to call functions on external programs.
    /// # Errors
    /// Returns a [`StateError`] if the call fails.
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_function<T: BorshDeserialize>(
        &self,
        function_name: &str,
        args: &Params,
        max_units: i64,
    ) -> Result<T, StateError> {
        #[link(wasm_import_module = "program")]
        extern "C" {
            #[link_name = "call_program"]
            fn ffi(ptr: *const u8, len: usize) -> *const u8;
        }

        let args = CallProgramArgs {
            target_id: self.id(),
            function: function_name.as_bytes(),
            args_ptr: args,
            max_units,
        };

        let args_bytes = borsh::to_vec(&args).map_err(|_| StateError::Serialization)?;

        let ptr = unsafe { ffi(args_bytes.as_ptr(), args_bytes.len()) };

        let bytes = into_bytes(ptr).unwrap_or_default();

        borsh::from_slice(&bytes).map_err(|_| StateError::Deserialization)
    }
}

#[derive(BorshSerialize)]
struct CallProgramArgs<'a> {
    target_id: &'a [u8],
    function: &'a [u8],
    args_ptr: &'a [u8],
    max_units: i64,
}
