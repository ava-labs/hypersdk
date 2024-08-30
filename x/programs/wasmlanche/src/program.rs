// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use crate::{memory::HostPtr, types::Address, Gas};
use borsh::{BorshDeserialize, BorshSerialize};
use displaydoc::Display;

/// An error that is returned from call to public functions.
#[derive(Debug, Display, BorshSerialize, BorshDeserialize, PartialEq, Eq)]
#[repr(u8)]
#[non_exhaustive]
#[borsh(use_discriminant = true)]
pub enum ExternalCallError {
    /// an error happened during execution
    ExecutionFailure = 0,
    /// the call panicked
    CallPanicked = 1,
    /// not enough fuel to cover the execution
    OutOfFuel = 2,
    /// insufficient funds
    InsufficientFunds = 3,
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
    /// # use wasmlanche::{Address, Program};
    /// #
    /// # let program_id = [0; Address::LEN];
    /// # let target: Program = borsh::from_slice(&program_id).unwrap();
    /// let increment = 10;
    /// let params = borsh::to_vec(&increment).unwrap();
    /// let max_units = 1000000;
    /// let value = 0;
    /// let has_incremented: bool = target.call_function("increment", &params, max_units, value)?;
    /// assert!(has_incremented);
    /// # Ok::<(), wasmlanche::ExternalCallError>(())
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
}

#[derive(BorshSerialize)]
struct CallProgramArgs<'a> {
    target: &'a Program,
    function: &'a [u8],
    args: &'a [u8],
    max_units: Gas,
    max_value: u64,
}
