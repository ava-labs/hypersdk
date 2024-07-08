use borsh::BorshSerialize;
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys};
use wasmlanche_sdk::{ExternalCallError, Program};

#[derive(Debug, BorshSerialize)]
pub enum Error {
    AlreadyScheduled,
    NotScheduled,
    TooEarly,
    AlreadyExecuted,
    ExternalCall(ExternalCallError),
}

#[state_keys]
pub enum StateKeys {
    Time,
    Program,
    Function,
    Args,
    Executed,
}

const LOCK_TIME: u64 = 3600;

#[public]
pub fn schedule(
    context: Context<StateKeys>,
    to: Program,
    function: String,
    data: Vec<u8>,
) -> Result<(), Error> {
    let program = context.program();

    if program
        .state()
        .get::<u64>(StateKeys::Time)
        .expect("state corrupt")
        .is_some()
    {
        return Err(Error::AlreadyScheduled);
    }

    program
        .state()
        .store_by_key(StateKeys::Time, &(context.timestamp() + LOCK_TIME))
        .expect("state corrupt");

    program
        .state()
        .store_by_key(StateKeys::Program, &to)
        .expect("state corrupt");

    program
        .state()
        .store_by_key(StateKeys::Function, &function)
        .expect("state corrupt");

    program
        .state()
        .store_by_key(StateKeys::Args, &data)
        .expect("state corrupt");

    Ok(())
}

#[public]
pub fn execute(context: Context<StateKeys>, max_units: u64) -> Result<Vec<u8>, Error> {
    let program = context.program;

    if program
        .state()
        .get::<bool>(StateKeys::Executed)
        .expect("state corrupt")
    {
        return Err(Error::AlreadyExecuted);
    }

    let time = program
        .state()
        .get::<u64>(StateKeys::Time)
        .expect("state corrupt")
        .ok_or(Error::NotScheduled)?;

    if time > context.timestamp() {
        return Err(Error::TooEarly);
    }

    let called_program = program
        .state()
        .get::<Program>(StateKeys::Program)
        .expect("state corrupt")
        .unwrap();

    let function = program
        .state()
        .get::<String>(StateKeys::Function)
        .expect("state corrupt")
        .unwrap();

    let args = program
        .state()
        .get::<Vec<u8>>(StateKeys::Args)
        .expect("state corrupt")
        .unwrap();

    called_program
        .call_function_bytes(&function, args.as_slice(), max_units)
        .map_err(Error::ExternalCall)
}
