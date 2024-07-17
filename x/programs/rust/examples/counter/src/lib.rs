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
    use simulator::{Endpoint, Key, Param, Step, TestContext};

    const PROGRAM_PATH: &str = env!("PROGRAM_PATH");

    #[test]
    fn init_program() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");
        let alice = String::from("alice");

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner.clone())))
            .unwrap();

        simulator
            .run_step(&Step::create_key(Key::Ed25519(alice)))
            .unwrap();

        simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap();
    }

    #[test]
    fn increment() {
        let mut simulator = simulator::ClientBuilder::new().try_build().unwrap();

        let owner = String::from("owner");
        let bob_key = Key::Ed25519(String::from("bob"));
        let bob_key_param = Param::Key(bob_key.clone());

        simulator
            .run_step(&Step::create_key(Key::Ed25519(owner.clone())))
            .unwrap();

        simulator.run_step(&Step::create_key(bob_key)).unwrap();

        let counter_id = simulator
            .run_step(&Step::create_program(PROGRAM_PATH))
            .unwrap()
            .id;

        let test_context = TestContext::from(counter_id);

        simulator
            .run_step(&Step {
                endpoint: Endpoint::Execute,
                method: "inc".into(),
                max_units: 1000000,
                params: vec![
                    test_context.clone().into(),
                    bob_key_param.clone(),
                    10u64.into(),
                ],
            })
            .unwrap();

        let value = simulator
            .run_step(&Step {
                endpoint: Endpoint::ReadOnly,
                method: "get_value".into(),
                max_units: 0,
                params: vec![test_context.clone().into(), bob_key_param.clone()],
            })
            .unwrap()
            .result
            .response::<u64>()
            .unwrap();
        assert_eq!(value, 10);
    }
}
