use wasmlanche_sdk::{public, types::Address, Context, ExternalCallContext, Program};

#[public]
pub fn inc(_: Context, external: Program, address: Address) {
    let ctx = ExternalCallContext::new(external, 1_000_000);
    counter::inc(ctx, address, 1);
}

#[public]
pub fn get_value(_: Context, external: Program, address: Address) -> u64 {
    let ctx = ExternalCallContext::new(external, 1_000_000);
    counter::get_value(ctx, address)
}

#[cfg(test)]
mod tests {
    use simulator::{ClientBuilder, Endpoint, Key, Param, Step};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn inc_and_get_value() {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let counter_path = PROGRAM_PATH
            .replace("counter-external", "counter")
            .replace("counter_external", "counter");

        let owner = String::from("owner");
        let owner_key = Key::Ed25519(owner.clone());

        simulator
            .run_step(&owner, &Step::create_key(owner_key.clone()))
            .unwrap();
        let owner_key = Param::Key(owner_key);

        let counter_external = simulator
            .run_step(&owner, &Step::create_program(PROGRAM_PATH))
            .expect("should be able to create this program")
            .id;
        let counter_external = Param::Id(counter_external);

        let counter = simulator
            .run_step(&owner, &Step::create_program(counter_path))
            .expect("should be able to create the counter")
            .id;
        let counter = Param::Id(counter);

        let params = vec![counter_external, counter.clone(), owner_key.clone()];

        simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::Execute,
                    method: "inc".into(),
                    max_units: 100_000_000,
                    params: params.clone(),
                },
            )
            .expect("call inc");

        let response = simulator
            .run_step(
                &owner,
                &Step {
                    endpoint: Endpoint::ReadOnly,
                    method: "get_value".into(),
                    max_units: 1_000_000,
                    params,
                },
            )
            .expect("call get_value")
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(response, 1u64);
    }
}
