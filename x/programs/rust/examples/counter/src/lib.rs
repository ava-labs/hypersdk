#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, types::Address, Gas, Program};

#[state_keys]
pub enum StateKeys {
    /// The count of this program. Key prefix 0x0 + address
    Counter(Address),
}

type Count = u64;

/// Increments the count at the address by the amount.
#[public]
pub fn inc(context: Context<StateKeys>, to: Address, amount: Count) -> bool {
    let counter = amount + get_value_internal(&context, to);
    let Context { program, .. } = context;

    program
        .state()
        .store(StateKeys::Counter(to), &counter)
        .expect("failed to store counter");

    true
}

/// Increments the count at the address by the amount for an external program.
#[public]
pub fn inc_external(
    _: Context,
    target: Program,
    max_units: Gas,
    of: Address,
    amount: Count,
) -> bool {
    let args = borsh::to_vec(&(of, amount)).unwrap();
    target.call_function("inc", &args, max_units).unwrap()
}

/// Gets the count at the address.
#[public]
pub fn get_value(context: Context<StateKeys>, of: Address) -> Count {
    get_value_internal(&context, of)
}

#[cfg(not(feature = "bindings"))]
fn get_value_internal(context: &Context<StateKeys>, of: Address) -> Count {
    context
        .program
        .state()
        .get(StateKeys::Counter(of))
        .expect("state corrupt")
        .unwrap_or_default()
}

/// Gets the count at the address for an external program.
#[public]
pub fn get_value_external(_: Context, target: Program, max_units: Gas, of: Address) -> Count {
    let args = borsh::to_vec(&of).unwrap();
    target.call_function("get_value", &args, max_units).unwrap()
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let alice_key = Param::Key(Key::Ed25519(String::from("alice")));

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Key,
                    method: "key_create".into(),
                    params: vec![alice_key.clone()],
                    max_units: 0,
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 1000000,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap();
    }

    #[test]
    fn increment() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let bob_key = Param::Key(Key::Ed25519(String::from("bob")));

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Key,
                    method: "key_create".into(),
                    params: vec![bob_key.clone()],
                    max_units: 0,
                },
            )
            .unwrap();

        let counter_id = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 1000000,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap()
            .id;

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "inc".into(),
                    max_units: 1000000,
                    params: vec![counter_id.into(), bob_key.clone(), 10u64.into()],
                },
            )
            .unwrap();

        let value = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_value".into(),
                    max_units: 0,
                    params: vec![counter_id.into(), bob_key.clone()],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(value, 10);
    }

    #[test]
    fn external_call() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner_key = String::from("owner");
        let bob_key = Param::Key(Key::Ed25519(String::from("bob")));

        simulator
            .run_step(
                &owner_key,
                &Step::create_key(Key::Ed25519(owner_key.clone())),
            )
            .unwrap();

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Key,
                    method: "key_create".into(),
                    params: vec![bob_key.clone()],
                    max_units: 0,
                },
            )
            .unwrap();

        let counter1_id = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 1000000,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap()
            .id;

        let counter2_id = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "program_create".into(),
                    max_units: 1000000,
                    params: vec![Param::String(PROGRAM_PATH.into())],
                },
            )
            .unwrap()
            .id;

        let value = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_value".into(),
                    max_units: 0,
                    params: vec![counter2_id.into(), bob_key.clone()],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(value, 0);

        simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "inc_external".into(),
                    max_units: 100_000_000,
                    params: vec![
                        counter1_id.into(),
                        counter2_id.into(),
                        1_000_000.into(),
                        bob_key.clone(),
                        10.into(),
                    ],
                },
            )
            .unwrap();

        let value = simulator
            .run_step(
                &owner_key,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_value_external".into(),
                    max_units: 0,
                    params: vec![
                        counter1_id.into(),
                        counter2_id.into(),
                        1_000_000.into(),
                        bob_key.clone(),
                    ],
                },
            )
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(value, 10);
    }
}
