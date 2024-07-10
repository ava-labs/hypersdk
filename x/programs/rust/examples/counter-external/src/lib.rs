use wasmlanche_sdk::{public, types::Address, Context, ExternalCallContext, Program};

#[public]
pub fn inc(_: Context, external: Program, address: Address) {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0);
    counter::inc(ctx, address, 1);
}

#[public]
pub fn get_value(_: Context, external: Program, address: Address) -> u64 {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0);
    counter::get_value(ctx, address)
}

#[cfg(test)]
mod tests {
    use simulator::{ClientBuilder, Endpoint, Param, Step, TestContext};
    use wasmlanche_sdk::types::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn inc_and_get_value() {
        let mut simulator = ClientBuilder::new().try_build().unwrap();

        let counter_path = PROGRAM_PATH
            .replace("counter-external", "counter")
            .replace("counter_external", "counter");

        let owner = Address::from_str("owner");

        let counter_external = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .expect("should be able to create this program")
            .id;

        let counter = simulator
            .run_step(&Step::create_program(counter_path))
            .expect("should be able to create the counter")
            .id;
        let counter = Param::Id(counter);

        let params = vec![
            TestContext::from(counter_external).into(),
            counter.clone(),
            owner.into(),
        ];

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "inc".into(),
                max_units: 100_000_000,
                params: params.clone(),
            })
            .expect("call inc");

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "get_value".into(),
                max_units: 1_000_000,
                params,
            })
            .expect("call get_value")
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(response, 1);
    }
}
