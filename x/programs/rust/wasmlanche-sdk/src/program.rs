use borsh::{BorshDeserialize, BorshSerialize};

use crate::{memory::to_host_ptr, state::Error as StateError, state::State, Params};

/// Represents the current Program in the context of the caller. Or an external
/// program that is being invoked.
#[derive(Clone, Copy, BorshDeserialize, BorshSerialize)]
pub struct Program([u8; Self::LEN]);

impl Program {
    /// The length of ids.ID
    pub const LEN: usize = 32;

    /// Returns the id of the program.
    #[must_use]
    pub fn id(&self) -> &[u8; Self::LEN] {
        &self.0
    }

    #[must_use]
    pub(crate) fn new(id: [u8; Self::LEN]) -> Self {
        Self(id)
    }

    /// Returns a State object that can be used to interact with persistent
    /// storage exposed by the host.
    #[must_use]
    pub fn state(&self) -> State {
        State::new(Program::new(*self.id()))
    }

    /// Attempts to call a function `name` with `args` on the given program. This method
    /// is used to call functions on external programs.
    /// # Errors
    /// Returns a [`StateError`] if the call fails.
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_function(
        &self,
        function_name: &str,
        args: Params,
        max_units: i64,
    ) -> Result<i64, StateError> {
        // flatten the args into a single byte vector
        let target = to_host_ptr(self.id())?;
        let function = to_host_ptr(function_name.as_bytes())?;
        let args = args.into_host_ptr()?;

        Ok(unsafe { _call_program(target, function, args, max_units) })
    }
}

#[link(wasm_import_module = "program")]
extern "C" {
    #[link_name = "call_program"]
    fn _call_program(target_id: i64, function: i64, args_ptr: i64, max_units: i64) -> i64;
}
