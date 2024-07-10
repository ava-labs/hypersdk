use borsh::{BorshDeserialize, BorshSerialize};
#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, DeferDeserialize, ExternalCallError, Program};

#[derive(Debug, BorshSerialize, BorshDeserialize, PartialEq)]
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
pub fn execute(context: Context<StateKeys>, max_units: u64) -> Result<DeferDeserialize, Error> {
    let program = context.program();

    if program
        .state()
        .get::<bool>(StateKeys::Executed)
        .expect("state corrupt")
        .is_some_and(|executed| executed)
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

    program
        .state()
        .store_by_key(StateKeys::Executed, &true)
        .unwrap();

    called_program
        .call_function(&function, args.as_slice(), max_units)
        .map_err(Error::ExternalCall)
}

#[public]
pub fn next_execution(context: Context<StateKeys>) -> Option<u64> {
    context
        .program()
        .state()
        .get(StateKeys::Time)
        .expect("state corrupt")
}

#[cfg(test)]
mod tests {
    use super::{Error, LOCK_TIME};
    use simulator::{Endpoint, Id, Step, TestContext};
    use wasmlanche_sdk::DeferDeserialize;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn cannot_pre_execute() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(program_id);

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), u64::MAX.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), Error>>()
            .unwrap();

        assert_eq!(res, Err(Error::NotScheduled));
    }

    #[test]
    fn schedule_in_the_future() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);
        let timestamp = 123456;
        test_context.timestamp = timestamp;

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "schedule".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    Id::default().into(),
                    String::new().into(),
                    Vec::new().into(),
                ],
            })
            .unwrap();

        let next_execution: Option<u64> = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "next_execution".to_string(),
                max_units: 0,
                params: vec![test_context.into()],
            })
            .unwrap()
            .result
            .response()
            .unwrap();

        assert_eq!(next_execution, Some(timestamp + LOCK_TIME));
    }

    #[test]
    fn delayed_execution() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);
        let timestamp = 123456;
        test_context.timestamp = timestamp;

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "schedule".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    program_id.into(),
                    String::from("next_execution").into(),
                    Vec::new().into(),
                ],
            })
            .unwrap();

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 1000000u64.into()],
            })
            .unwrap()
            .result
            .response::<Result<(), Error>>()
            .unwrap();

        assert_eq!(res, Err(Error::TooEarly));

        test_context.timestamp = timestamp + LOCK_TIME;

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 1000000u64.into()],
            })
            .unwrap()
            .result
            .response::<Result<DeferDeserialize, Error>>()
            .unwrap();

        assert!(res.is_ok());
    }

    #[test]
    fn cannot_re_execute() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);
        let timestamp = 123456;
        test_context.timestamp = timestamp;

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "schedule".to_string(),
                max_units: u64::MAX,
                params: vec![
                    test_context.clone().into(),
                    program_id.into(),
                    String::new().into(),
                    Vec::new().into(),
                ],
            })
            .unwrap();

        test_context.timestamp = timestamp + LOCK_TIME;

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 1000000u64.into()],
            })
            .unwrap();

        let res = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "execute".to_string(),
                max_units: u64::MAX,
                params: vec![test_context.clone().into(), 1000000u64.into()],
            })
            .unwrap()
            .result
            .response::<Result<DeferDeserialize, Error>>()
            .unwrap();

        assert!(matches!(res, Err(Error::AlreadyExecuted)));
    }
}
