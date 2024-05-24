use wasmlanche_sdk::{params, public, state_keys, types::Address, Context, Program};

#[state_keys]
pub enum StateKeys {
    /// The count of this program. Key prefix 0x0 + address
    Counter(Address),
}

/// Initializes the program address a count of 0.
#[public]
pub fn initialize_address(context: Context<StateKeys>, address: Address) -> bool {
    let Context { program, .. } = context;

    if program
        .state()
        .get::<i64>(StateKeys::Counter(address))
        .is_ok()
    {
        panic!("counter already initialized for address")
    }

    program
        .state()
        .store(StateKeys::Counter(address), &0_i64)
        .expect("failed to store counter");

    true
}

/// Increments the count at the address by the amount.
#[public]
pub fn inc(context: Context<StateKeys>, to: Address, amount: i64) -> bool {
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
pub fn inc_external(_: Context, target: Program, max_units: i64, of: Address, amount: i64) -> bool {
    let params = params!(&of, &amount).unwrap();
    target.call_function("inc", &params, max_units).unwrap()
}

/// Gets the count at the address.
#[public]
pub fn get_value(context: Context<StateKeys>, of: Address) -> i64 {
    get_value_internal(&context, of)
}

fn get_value_internal(context: &Context<StateKeys>, of: Address) -> i64 {
    context
        .program
        .state()
        .get(StateKeys::Counter(of))
        .expect("failed to get counter")
}

/// Gets the count at the address for an external program.
#[public]
pub fn get_value_external(_: Context, target: Program, max_units: i64, of: Address) -> i64 {
    let params = params!(&of).unwrap();
    target
        .call_function("get_value", &params, max_units)
        .unwrap()
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Plan, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut simulator = simulator::Client::new_stdin();

        let owner_key = String::from("owner");
        let alice_key = Param::Key(Key::Ed25519(String::from("alice")));

        let mut plan = Plan::new(&owner_key);

        plan.add_step(Step::create_key(Key::Ed25519(owner_key.clone())));

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![alice_key.clone()],
            max_units: 0,
        });

        let counter1_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 1000000,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "initialize_address".into(),
            max_units: 1000000,
            params: vec![counter1_id.into(), alice_key.clone()],
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );
    }

    #[test]
    fn increment() {
        let mut simulator = simulator::Client::new_stdin();

        let owner_key = String::from("owner");
        let bob_key = Param::Key(Key::Ed25519(String::from("bob")));

        let mut plan = Plan::new(&owner_key);

        plan.add_step(Step::create_key(Key::Ed25519(owner_key.clone())));

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![bob_key.clone()],
            max_units: 0,
        });

        let counter_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 1000000,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "initialize_address".into(),
            max_units: 1000000,
            params: vec![counter_id.into(), bob_key.clone()],
        });

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "inc".into(),
            max_units: 1000000,
            params: vec![counter_id.into(), bob_key.clone(), 10.into()],
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );

        let value = bytes_to_u64(
            simulator
                .run_step(
                    &owner_key,
                    Step {
                        endpoint: Endpoint::ReadOnly,
                        method: "get_value".into(),
                        max_units: 0,
                        params: vec![counter_id.into(), bob_key.clone()],
                    },
                )
                .unwrap()
                .result
                .response,
        );
        assert_eq!(value, 10);
    }

    #[test]
    #[ignore = "need to fix params macro"]
    fn external_call() {
        let mut simulator = simulator::Client::new_stdin();

        let owner_key = String::from("owner");
        let bob_key = Param::Key(Key::Ed25519(String::from("bob")));

        let mut plan = Plan::new(&owner_key);

        plan.add_step(Step::create_key(Key::Ed25519(owner_key.clone())));

        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![bob_key.clone()],
            max_units: 0,
        });

        let counter1_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 1000000,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });

        let counter2_id = plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 1000000,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "initialize_address".into(),
            max_units: 1000000,
            params: vec![counter2_id.into(), bob_key.clone()],
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );

        let value = bytes_to_u64(
            simulator
                .run_step(
                    &owner_key,
                    Step {
                        endpoint: Endpoint::ReadOnly,
                        method: "get_value".into(),
                        max_units: 0,
                        params: vec![counter2_id.into(), bob_key.clone()],
                    },
                )
                .unwrap()
                .result
                .response,
        );
        assert_eq!(value, 0);

        simulator
            .run_step(
                &owner_key,
                Step {
                    endpoint: Endpoint::Execute,
                    method: "inc_external".into(),
                    max_units: 100000000,
                    params: vec![
                        counter1_id.into(),
                        counter2_id.into(),
                        1000000.into(),
                        bob_key.clone(),
                        10.into(),
                    ],
                },
            )
            .unwrap();

        let value = bytes_to_u64(
            simulator
                .run_step(
                    &owner_key,
                    Step {
                        endpoint: Endpoint::ReadOnly,
                        method: "get_value_external".into(),
                        max_units: 0,
                        params: vec![
                            counter1_id.into(),
                            counter2_id.into(),
                            1000000.into(),
                            bob_key.clone(),
                        ],
                    },
                )
                .unwrap()
                .result
                .response,
        );
        assert_eq!(value, 10);
    }

    fn bytes_to_u64(bytes: Vec<u8>) -> u64 {
        u64::from_le_bytes(bytes.try_into().unwrap())
    }
}
