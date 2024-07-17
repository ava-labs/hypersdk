use wasmlanche_sdk::{public, Address, Context};

#[public]
pub fn get_timestamp(context: Context) -> u64 {
    context.timestamp()
}

#[public]
pub fn get_height(context: Context) -> u64 {
    context.height()
}

#[public]
pub fn get_actor(context: Context) -> Address {
    context.actor()
}

#[cfg(test)]
mod tests {
    use simulator::{ClientBuilder, Endpoint, Step, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn can_set_timestamp() {
        let mut simulator = simulator::build().unwrap();

        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let timestamp = 100;
        let mut test_context = TestContext::from(program_id);
        test_context.timestamp = timestamp;

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "get_timestamp".into(),
                max_units: 1000000,
                params: vec![test_context.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(response, timestamp);
    }

    #[test]
    fn can_set_height() {
        let mut simulator = simulator::build().unwrap();


        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let height = 1000;
        let mut test_context = TestContext::from(program_id);
        test_context.height = height;

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "get_height".into(),
                max_units: 1000000,
                params: vec![test_context.into()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(response, height);
    }

    #[test]
    fn can_set_actor() {
        let mut simulator = simulator::build().unwrap();


        let program_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let mut test_context = TestContext::from(program_id);
        test_context.actor = Address::new([1; 33]);

        let response = simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "get_actor".into(),
                max_units: 1000000,
                params: vec![test_context.into()],
            })
            .unwrap()
            .result
            .response::<Address>()
            .unwrap();

        assert_ne!(response, Address::default());
    }
}
