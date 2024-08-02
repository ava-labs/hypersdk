use wasmlanche_sdk::{public, Address, Context, ExternalCallContext, Program};

#[public]
pub fn inc(_: &mut Context, external: Program, address: Address) {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0);
    counter::inc(&ctx, address, 1);
}

#[public]
pub fn get_value(_: &mut Context, external: Program, address: Address) -> u64 {
    let ctx = ExternalCallContext::new(external, 1_000_000, 0);
    counter::get_value(&ctx, address)
}

#[cfg(test)]
mod tests {
    use simulator::Simulator;

    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn inc_and_get_value() {
        let mut simulator = Simulator::new();

        let counter_path = PROGRAM_PATH
            .replace("counter-external", "counter")
            .replace("counter_external", "counter");

        let owner = Address::new([1; 33]);

        let counter_external = simulator.create_program(PROGRAM_PATH).program().unwrap();

        let counter = simulator.create_program(&counter_path).program().unwrap();

        let res = simulator.execute(counter_external, "inc", (counter, owner), 100_000_000);
        // TODO check err

        let response = simulator
            .execute(counter_external, "get_value", (counter, owner), 100_000_000)
            .result::<u64>()
            .unwrap();

        assert_eq!(response, 1);
    }
}
