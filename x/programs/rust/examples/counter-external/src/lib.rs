use wasmlanche_sdk::{public, Address, Context, ExternalCallContext, Program};

#[public]
pub fn inc(_: Context, external: Program, address: Address) {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0);
    counter::inc(&ctx, address, 1);
}

#[public]
pub fn get_value(_: Context, external: Program, address: Address) -> u64 {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0);
    counter::get_value(&ctx, address)
}

#[cfg(test)]
mod tests {
    use simulator::{build_simulator, Param, TestContext};

    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn inc_and_get_value() {
        let mut simulator = build_simulator();

        let counter_path = PROGRAM_PATH
            .replace("counter-external", "counter")
            .replace("counter_external", "counter");

        let owner = Address::new([1; 33]);

        let counter_external = simulator
            .create_program(PROGRAM_PATH)
            .expect("should be able to create this program")
            .id;

        let counter = simulator
            .create_program(counter_path)
            .expect("should be able to create the counter")
            .id;

        let counter = Param::Id(counter);

        let params = vec![
            TestContext::from(counter_external).into(),
            counter.clone(),
            owner.into(),
        ];

        simulator
            .execute("inc".into(), params.clone(), 100_000_000)
            .expect("call inc");

        let response = simulator
            .read("get_value".into(), params)
            .expect("call get_value")
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(response, 1);
    }
}
