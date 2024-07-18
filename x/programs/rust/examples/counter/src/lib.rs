#[cfg(not(feature = "bindings"))]
use wasmlanche_sdk::Context;
use wasmlanche_sdk::{public, state_keys, Address};

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

    context
        .store_by_key(StateKeys::Counter(to), &counter)
        .expect("failed to store counter");

    true
}

/// Gets the count at the address.
#[public]
pub fn get_value(context: Context<StateKeys>, of: Address) -> Count {
    get_value_internal(&context, of)
}

#[cfg(not(feature = "bindings"))]
fn get_value_internal(context: &Context<StateKeys>, of: Address) -> Count {
    context
        .get(StateKeys::Counter(of))
        .expect("state corrupt")
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use simulator::{build_simulator, TestContext};
    use wasmlanche_sdk::Address;

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut simulator = build_simulator().unwrap();

        simulator.create_program(PROGRAM_PATH).unwrap();
    }

    #[test]
    fn increment() {
        let mut simulator = build_simulator().unwrap();

        let bob = Address::new([1; 33]);

        let counter_id = simulator.create_program(PROGRAM_PATH).unwrap().id;

        let test_context = TestContext::from(counter_id);

        simulator.execute(
            "inc".into(),
            vec![test_context.clone().into(), bob.into(), 10u64.into()],
            1000000,
        );

        let value = simulator
            .read("get_value".into(), vec![test_context.into(), bob.into()])
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();

        assert_eq!(value, 10);
    }
}
