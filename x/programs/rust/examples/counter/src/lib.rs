use wasmlanche_sdk::{public, state_keys, types::Address, Context, Program};

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
    max_units: i64,
    of: Address,
    amount: Count,
) -> bool {
    target
        .call_function("inc", (of, amount), max_units)
        .unwrap()
}

/// Gets the count at the address.
#[public]
pub fn get_value(context: Context<StateKeys>, of: Address) -> Count {
    get_value_internal(&context, of)
}

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
pub fn get_value_external(_: Context, target: Program, max_units: i64, of: Address) -> Count {
    target.call_function("get_value", of, max_units).unwrap()
}

#[cfg(test)]
mod tests {
    use simulator::{Endpoint, Key, Param, Plan, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut simulator = simulator::ClientBuilder::create().unwrap();

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

        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 1000000,
            params: vec![Param::String(PROGRAM_PATH.into())],
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );
    }

    #[test]
    fn increment() {
        let mut simulator = simulator::ClientBuilder::create().unwrap();

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
            method: "inc".into(),
            max_units: 1000000,
            params: vec![counter_id.into(), bob_key.clone(), 10.into()],
        });

        let plan_responses = simulator.run_plan(plan).unwrap();

        assert!(
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );

        let value = simulator
            .run_step::<u64>(
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
            .response;
        assert_eq!(value, 10);
    }

    #[test]
    #[ignore = "need to fix params macro"]
    fn external_call() {
        let mut simulator = simulator::ClientBuilder::create().unwrap();

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
            plan_responses.iter().all(|resp| resp.base.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.base.error.as_ref())
                .next()
        );

        let value = simulator
            .run_step::<u64>(
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
            .response;
        assert_eq!(value, 0);

        simulator
            .run_step::<bool>(
                &owner_key,
                &Step {
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

        let value = simulator
            .run_step::<u64>(
                &owner_key,
                &Step {
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
            .response;
        assert_eq!(value, 10);
    }
}
