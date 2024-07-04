use wasmlanche_sdk::{public, state_keys, Context};

#[state_keys]
pub enum StateKeys {
    LastTimestamp,
    LastHeight,
    LastActor,
}

#[public]
pub fn can_change_timestamp(context: Context<StateKeys>) {
    let new_timestamp = context.timestamp;
    match context
        .program
        .state()
        .get(StateKeys::LastTimestamp)
        .unwrap()
    {
        Some(timestamp) => {
            assert!(new_timestamp > timestamp)
        }
        None => {
            context
                .program
                .state()
                .store(StateKeys::LastTimestamp, &new_timestamp)
                .unwrap();
        }
    }
}

#[public]
pub fn can_change_height(context: Context<StateKeys>) {
    let new_height = context.height;
    match context.program.state().get(StateKeys::LastHeight).unwrap() {
        Some(height) => assert!(new_height > height),
        None => {
            context
                .program
                .state()
                .store(StateKeys::LastHeight, &new_height)
                .unwrap();
        }
    }
}

#[public]
pub fn can_change_actor(context: Context<StateKeys>) {
    let new_actor = context.actor;
    match context.program.state().get(StateKeys::LastActor).unwrap() {
        Some(actor) => assert!(new_actor != actor),
        None => {
            context
                .program
                .state()
                .store(StateKeys::LastActor, &new_actor)
                .unwrap();
        }
    }
}

#[cfg(test)]
mod tests {
    use simulator::{ClientBuilder, Endpoint, Key, Step, TestContext};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn context_injection() {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");

        simulator
            .run_step(&owner, &Step::create_key(Key::Ed25519(owner.clone())))
            .unwrap();

        let program_id = simulator
            .run_step(&owner, &Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let context = TestContext::default().with_program_id(program_id);

        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "can_change_timestamp".into(),
                    max_units: 1000000,
                    params: vec![context.clone().with_timestamp(100).into()],
                },
            )
            .unwrap();
        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "can_change_timestamp".into(),
                    max_units: 1000000,
                    params: vec![context.clone().with_timestamp(101).into()],
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "can_change_height".into(),
                    max_units: 1000000,
                    params: vec![context.clone().with_height(10_000).into()],
                },
            )
            .unwrap();
        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "can_change_height".into(),
                    max_units: 1000000,
                    params: vec![context.clone().with_height(40_000).into()],
                },
            )
            .unwrap();

        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "can_change_actor".into(),
                    max_units: 1000000,
                    params: vec![context.clone().into()],
                },
            )
            .unwrap();

        let bob_key = Key::Ed25519(String::from("bob"));
        simulator
            .run_step(&owner, &Step::create_key(bob_key.clone()))
            .unwrap();

        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "can_change_actor".into(),
                    max_units: 1000000,
                    params: vec![context.clone().with_actor_key(bob_key).into()],
                },
            )
            .unwrap();
    }
}
