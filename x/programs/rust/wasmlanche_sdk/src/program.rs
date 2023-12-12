use borsh::{BorshDeserialize, BorshSerialize};

use crate::{errors::StateError, host::call_program, state::State};

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

    /// Attempts to call another program `target` from this program `caller`.
    /// # Errors
    /// Returns a `StateError` if the call fails.
    /// # Safety
    /// The caller must ensure that `function_name` + `args` point to valid memory locations.
    pub fn call_program(
        &self,
        target: &Program,
        max_units: i64,
        function_name: &str,
        args: Vec<Vec<u8>>,
    ) -> Result<i64, StateError> {
        // flatten the args into a single byte vector
        let args = args.into_iter().flatten().collect::<Vec<u8>>();
        call_program(self, target, max_units, function_name, &args)
    }
}

/// Serialize every parameter into a byte vector and return a vector of byte vectors
#[macro_export]
macro_rules! params {
    ($($param:expr),*) => {
        vec![$(wasmlanche_sdk::program::serialize_params($param).unwrap(),)*]
    };
}

/// Serializes the parameter into a byte vector.
/// # Errors
/// Will return an error if the parameter cannot be serialized.
pub fn serialize_params<T>(param: &T) -> Result<Vec<u8>, std::io::Error>
where
    T: BorshSerialize,
{
    let bytes = prepend_length(&borsh::to_vec(param)?);
    Ok(bytes)
}

/// Returns a vector of bytes with the length of the argument prepended.
/// # Panics
/// Panics if the length of the argument cannot be converted to u32.
fn prepend_length(bytes: &[u8]) -> Vec<u8> {
    let mut len_bytes = u32::try_from(bytes.len())
        .expect("pointer out range")
        .to_be_bytes()
        .to_vec();
    len_bytes.extend(bytes);
    len_bytes
}
