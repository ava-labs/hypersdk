use borsh::{BorshDeserialize, BorshSerialize};

use crate::{
    host::call_program,
    state::{prepend_length, State},
};

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
        args: Vec<Vec<u8>>,
    ) -> i64 {
        // flatten the args into a single byte vector
        let args = args.into_iter().flatten().collect::<Vec<u8>>();
        call_program(self, target, max_units, function_name, &args)
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

#[macro_export]
macro_rules! serialize_params {
    ($($param:expr),*) => {
        {
            // the macro expands into this block. This will serialize every parameter
            // into a byte vector and return a vector of byte vectors.
            let mut params = Vec::new();
            $(
                params.push(wasmlanche_sdk::program::serialize_params(&$param).unwrap());
            )*

            params
        }
    };
}

pub fn serialize_params<T>(param: &T) -> Result<Vec<u8>, std::io::Error>
where
    T: BorshSerialize,
{
    let bytes = prepend_length(&borsh::to_vec(param)?);
    Ok(bytes)
}
