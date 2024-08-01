use wasmlanche_sdk::{public, Address, Context};

#[public]
pub fn get_timestamp(context: &mut Context) -> u64 {
    context.timestamp()
}

#[public]
pub fn get_height(context: &mut Context) -> u64 {
    context.height()
}

#[public]
pub fn get_actor(context: &mut Context) -> Address {
    context.actor()
}

#[cfg(test)]
mod tests {
    use simulator::{build_simulator, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn can_set_timestamp() {
        let mut simulator = build_simulator();

        let program_id = simulator.create_program(PROGRAM_PATH).unwrap().id;

        let timestamp = 100;
        let mut test_context = TestContext::from(program_id);
        test_context.timestamp = timestamp;
        let response = simulator
            .execute("get_timestamp".into(), vec![test_context.into()], 1000000)
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(response, timestamp);
    }

    #[test]
    fn can_set_height() {
        let mut simulator = build_simulator();

        let program_id = simulator.create_program(PROGRAM_PATH).unwrap().id;

        let height = 1000;
        let mut test_context = TestContext::from(program_id);
        test_context.height = height;

        let response = simulator
            .execute("get_height".into(), vec![test_context.into()], 1000000)
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(response, height);
    }

    #[test]
    fn can_set_actor() {
        let mut simulator = build_simulator();

        let program_id = simulator.create_program(PROGRAM_PATH).unwrap().id;

        let mut test_context = TestContext::from(program_id);
        test_context.actor = Address::new([1; 33]);

        let response = simulator
            .execute("get_actor".into(), vec![test_context.into()], 1000000)
            .unwrap()
            .result
            .response::<Address>()
            .unwrap();

        assert_ne!(response, Address::default());
    }
}
