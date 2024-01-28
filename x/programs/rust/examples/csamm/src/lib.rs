use std::cmp::Ordering;
use wasmlanche_sdk::{params, program::Program, public, state_keys, types::Address};

// Implementation of a CSAMM. This assumes a 1:1 peg between the two assets or one of them is going to be missing from the pool.

/// The program state keys.
#[state_keys]
enum StateKeys {
    // TODO this is also an LP token, could we reuse parts of the "token" program ?
    // TotalSupply,
    // Balance(Address),
    /// The vec of coins. Key prefix 0x0 + addresses
    // TODO Vec doesn't implement Copy, so can we not define dynamic array in the program storage ?
    // Coins([Address; 2]),
    CoinX,
    CoinY,
}

/// Initializes the program address a count of 0.
#[public]
fn constructor(program: Program, coin_x: Program, coin_y: Program) -> bool {
    if program.state().get::<Program, _>(StateKeys::CoinX).is_ok()
        || program.state().get::<Program, _>(StateKeys::CoinY).is_ok()
    {
        panic!("program already initialized")
    }

    program
        .state()
        .store(StateKeys::CoinX, &coin_x)
        .expect("failed to store coin_x");

    program
        .state()
        .store(StateKeys::CoinY, &coin_y)
        .expect("failed to store coin_y");

    true
}

// TODO manage this ID not safe lint. Return a pointer instead.
#[public]
fn coins(program: Program, i: u8) -> [u8; 32] {
    let coin = match i {
        0 => StateKeys::CoinX,
        1 => StateKeys::CoinY,
        _ => panic!("this coin does not exist"),
    };

    *program.state().get::<Program, _>(coin).unwrap().id()
}

fn token_x(program: Program) -> Program {
    program.state().get::<Program, _>(StateKeys::CoinX).unwrap()
}

fn token_y(program: Program) -> Program {
    program.state().get::<Program, _>(StateKeys::CoinY).unwrap()
}

fn balances(program: Program) -> (i64, i64) {
    let this = Address::new(*program.id());

    (
        program
            .state()
            .get::<Program, _>(StateKeys::CoinX)
            .unwrap()
            .call_function("get_balance", params!(&this), 10000)
            .unwrap(),
        program
            .state()
            .get::<Program, _>(StateKeys::CoinY)
            .unwrap()
            .call_function("get_balance", params!(&this), 10000)
            .unwrap(),
    )
}

/// exchange `amount` of token T for x amount of token T'.
/// amount > 0 sends tokens to the pool while amount < 0 pulls them.
#[public]
fn exchange(program: Program, amount: i64) {
    let sender = Address::new([0; 32]); // TODO how to get the program caller ?
    let this = Address::new(*program.id());

    // x + y = k
    // x + dx + y - dy = k
    // dy = x + dx + y - k
    // dy = x + dx + y - (x + y)
    // dy = dx

    // NOTE the extra granularity is good but can we avoid having to put a fixed max_units everytime ?
    // NOTE how to get a balance that is more than 32 bits over 0 ? For USD variants with 6 decimals, it's alright.
    // NOTE can we really have negative balances ?
    let (x, y) = balances(program);

    let dx = amount;
    let dy = dx;

    if dx > x {
        panic!("not enough x tokens in the pool!");
    }
    if dy > y {
        panic!("not enough y tokens in the pool!");
    }

    // NOTE rust makes it quite annoying to write constants for non-primitive types. It's not looking good for using pow operations
    match amount.cmp(&0) {
        Ordering::Greater => {
            token_x(program)
                .call_function("transfer", params!(&sender, &this, &dx), 10000)
                .unwrap();
            token_y(program)
                .call_function("transfer", params!(&this, &sender, &dy), 10000)
                .unwrap();
        }
        Ordering::Less => {
            token_y(program)
                .call_function("transfer", params!(&sender, &this, &dx), 10000)
                .unwrap();
            token_x(program)
                .call_function("transfer", params!(&this, &sender, &dy), 10000)
                .unwrap();
        }
        _ => panic!("amount == 0"),
    }
}

/// Add or remove liquidity. Both asset amounts should conform with the CSAMM properties.
#[public]
fn manage_liquidity(program: Program, dx: i64, dy: i64) {
    let sender = Address::new([0; 32]);
    let this = Address::new(*program.id());

    let (x, y) = balances(program);
    let k = x + y;

    assert_eq!(dx.signum(), dy.signum(), "either add or remove liquidity");
    match dx.cmp(&0) {
        Ordering::Greater => {
            token_x(program)
                .call_function("transfer", params!(&sender, &this, &dx), 10000)
                .unwrap();
            token_y(program)
                .call_function("transfer", params!(&sender, &this, &dy), 10000)
                .unwrap();
        }
        Ordering::Less => {
            token_x(program)
                .call_function("transfer", params!(&this, &sender, &-dx), 10000)
                .unwrap();
            token_y(program)
                .call_function("transfer", params!(&this, &sender, &-dy), 10000)
                .unwrap();
        }
        Ordering::Equal => panic!("amounts are negative"),
    };

    // post: is the equation still standing ?
    let (x, y) = balances(program);
    if x + y != k {
        panic!("CSAMM x + y = k property violated");
    }
}

#[cfg(test)]
mod tests {
    use serial_test::serial;
    use std::env;
    use wasmlanche_sdk::simulator::{
        self, id_from_step, Key, Operator, PlanResponse, Require, ResultAssertion,
    };

    // export SIMULATOR_PATH=/path/to/simulator
    // export PROGRAM_PATH=/path/to/program.wasm
    // cargo cargo test --package token --lib nocapture -- tests::test_token_plan --exact --nocapture --ignored
    #[test]
    #[serial]
    #[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
    fn test_liquidity() {
        use wasmlanche_sdk::simulator::{self, Endpoint, Key, Param, ParamType, Plan, Step};
        let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
        let simulator = simulator::Client::new(s_path);

        let owner_key = "owner";
        // create owner key in single step
        let resp = simulator
            .key_create::<PlanResponse>(owner_key, Key::Ed25519)
            .unwrap();
        assert_eq!(resp.error, None);

        // create multiple step test plan
        let mut plan = Plan::new(owner_key);

        // step 0: create program
        let p_path = env::var("PROGRAM_PATH").expect("PROGRAM_PATH not set");
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "program_create".into(),
            max_units: 0,
            params: vec![Param::new(ParamType::String, p_path.as_ref())],
            require: None,
        });

        // step 1: create alice key
        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![Param::new(ParamType::Key(Key::Ed25519), "alice_key")],
            max_units: 0,
            require: None,
        });

        // step 2: create bob key
        plan.add_step(Step {
            endpoint: Endpoint::Key,
            method: "key_create".into(),
            params: vec![Param::new(ParamType::Key(Key::Ed25519), "bob_key")],
            max_units: 0,
            require: None,
        });

        // step 3: init token program
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "init".into(),
            // program was created in step 0 so we can reference its id using the step_N identifier
            params: vec![Param::new(ParamType::Id, id_from_step(0).as_ref())],
            max_units: 10000,
            require: None,
        });

        // step 4: mint to alice
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "mint_to".into(),
            params: vec![
                Param::new(ParamType::Id, id_from_step(0).as_ref()),
                Param::new(ParamType::Key(Key::Ed25519), "alice_key"),
                Param::new(ParamType::U64, "1000"),
            ],
            max_units: 10000,
            require: None,
        });

        // step 5: transfer 100 from alice to bob
        plan.add_step(Step {
            endpoint: Endpoint::Execute,
            method: "transfer".into(),
            params: vec![
                Param::new(ParamType::Id, id_from_step(0).as_ref()),
                Param::new(ParamType::Key(Key::Ed25519), "alice_key"),
                Param::new(ParamType::Key(Key::Ed25519), "bob_key"),
                Param::new(ParamType::U64, "100"),
            ],
            max_units: 10000,
            require: None,
        });

        // run plan
        let plan_responses = simulator.run::<PlanResponse>(&plan).unwrap();

        // collect actual id of program from step 0
        let mut program_id = String::new();
        if let Some(step_0) = plan_responses.first() {
            program_id = step_0.result.id.clone().unwrap_or_default();
        }

        // ensure no errors
        assert!(
            plan_responses.iter().all(|resp| resp.error.is_none()),
            "error: {:?}",
            plan_responses
                .iter()
                .filter_map(|resp| resp.error.as_ref())
                .next()
        );

        // get total supply and assert result is expected
        let resp = simulator
            .read_only::<PlanResponse>(
                "owner",
                "get_total_supply",
                vec![Param::new(ParamType::Id, program_id.as_ref())],
                Some(Require {
                    result: ResultAssertion {
                        operator: Operator::NumericEq,
                        value: "123456789".into(),
                    },
                }),
            )
            .expect("failed to get total supply");
        assert_eq!(resp.error, None);

        // verify alice balance is 900
        let resp = simulator
            .read_only::<PlanResponse>(
                "owner",
                "get_balance",
                vec![
                    Param::new(ParamType::Id, program_id.as_ref()),
                    Param::new(ParamType::Key(Key::Ed25519), "alice_key"),
                ],
                Some(Require {
                    result: ResultAssertion {
                        operator: Operator::NumericEq,
                        value: "900".into(),
                    },
                }),
            )
            .expect("failed to get alice balance");
        assert_eq!(resp.error, None);

        // verify bob balance is 100
        let resp = simulator
            .read_only::<PlanResponse>(
                "owner",
                "get_balance",
                vec![
                    Param {
                        value: program_id.into(),
                        param_type: ParamType::Id,
                    },
                    Param {
                        value: "bob_key".into(),
                        param_type: ParamType::Key(Key::Ed25519),
                    },
                ],
                Some(Require {
                    result: ResultAssertion {
                        operator: Operator::NumericEq,
                        value: "100".into(),
                    },
                }),
            )
            .expect("failed to get bob balance");
        assert_eq!(resp.error, None);
    }

    #[test]
    #[serial]
    #[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
    fn test_create_program() {
        let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
        let simulator = simulator::Client::new(s_path);

        let owner_key = "owner";
        // create owner key in single step
        let resp = simulator
            .key_create::<PlanResponse>(owner_key, Key::Ed25519)
            .unwrap();
        assert_eq!(resp.error, None);

        let p_path = env::var("PROGRAM_PATH").expect("PROGRAM_PATH not set");
        // create a new program on chain.
        let resp = simulator
            .program_create::<PlanResponse>("owner", p_path.as_ref())
            .unwrap();
        assert_eq!(resp.error, None);
        assert_eq!(resp.result.id.is_some(), true);
    }
}
