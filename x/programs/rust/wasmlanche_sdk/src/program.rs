use crate::{host::call_program, state::State, types::is_primitive};
use borsh::{BorshSerialize, BorshDeserialize, to_vec};

/// Represents the current Program in the context of the caller. Or an external
/// program that is being invoked.
#[derive(Clone, Copy, BorshSerialize, BorshDeserialize)]
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
    pub fn call_program<T>(
        &self,
        target: &Program,
        max_units: i64,
        function_name: &str,
        args: &[Box<T>],
    )  -> i64 
    where 
    T: BorshSerialize {
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
/// Every argument is first prepended with its length and a flag indicating if
/// it is a primitive type.
fn marshal_args<T>(args: &[Box<T>]) -> Vec<u8>
where T: BorshSerialize {
    use std::mem::size_of;
    // Size of meta data for each argument
    const META_SIZE: usize = size_of::<i64>() + 1;

    // Calculate the total size of the combined byte slices
    let total_size = args.iter().map(|cow| to_vec(&cow).expect("Unable to Serialize").len() + META_SIZE).sum();

    // Create a mutable Vec<u8> to hold the combined bytes
    let mut bytes = Vec::with_capacity(total_size);

    for arg in args {
        // if we want to be efficient we dont need to add length of bytes if its an int
        let arg_bytes = to_vec(&arg).expect("Unable to Serialize");
        let len = arg_bytes.len() as i64;
        bytes.extend_from_slice(len.to_be_bytes().as_ref());
        
        // TODO: check if type of arg is a primitive. Don't think this is a good way to do it, especially
        // because some types can be 4 or 8 length
        bytes.extend_from_slice(&[u8::from(is_primitive::<T>())]);

        // Add the bytes of the argument
        bytes.extend_from_slice(arg_bytes.as_slice());
    }
    bytes
}
