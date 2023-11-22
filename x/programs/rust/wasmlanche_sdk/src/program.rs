use borsh::{BorshDeserialize, BorshSerialize};

use crate::{host::call_program, state::State, types::Argument};

/// Represents the current Program in the context of the caller. Or an external
/// program that is being invoked.
#[derive(Clone, Copy, BorshDeserialize, BorshSerialize)]
pub struct Program {
    id: i64,
}

impl Program {
    /// Returns the id of the program.
    #[must_use]
    pub fn id(self) -> i64 {
        self.id
    }

    /// Returns a State object that can be used to interact with persistent
    /// storage exposed by the host.
    #[must_use]
    pub fn state(&self) -> State {
        State::new(self.id.into())
    }

    /// Attempts to call another program `target` from this program `caller`.
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    #[must_use]
    pub fn call_program(
        &self,
        target: &Program,
        max_units: i64,
        function_name: &str,
        args: &[Box<dyn Argument>],
    ) -> i64 {
        call_program(
            self,
            target,
            max_units,
            function_name,
            marshal_args(args).as_ref(),
        )
    }
}

impl From<Program> for i64 {
    fn from(program: Program) -> Self {
        program.id()
    }
}

impl From<i64> for Program {
    fn from(value: i64) -> Self {
        Self { id: value }
    }
}

/// Marshals arguments into byte slice which can be unpacked by the host.
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
