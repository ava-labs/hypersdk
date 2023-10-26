use serial_test::serial;
use wasmlanche_sdk::simulator::{
    self, build, id_from_step, Endpoint, Key, Operator, Param, ParamType, Plan, Require,
    ResultAssertion, Step,
};

const SIMULATOR_BUILD_PATH: &str = "./tests/fixtures/simulator";
const SIMULATOR_GO_PATH: &str = "../../../cmd/simulator/simulator.go";

#[test]
#[serial]
fn test_token_plan() {
    build(SIMULATOR_GO_PATH, SIMULATOR_BUILD_PATH).expect("build simulator");
    let simulator = simulator::Client::new(SIMULATOR_BUILD_PATH);

    let owner_key = "owner";
    // create owner key in single step
    let resp = simulator.key_create(owner_key, Key::Ed25519).unwrap();
    assert_eq!(resp.error, None);

    // create multiple step test plan
    let mut plan = Plan::new(owner_key);

    // step 0: create program
    plan.add_step(Step {
        endpoint: Endpoint::Execute,
        method: "program_create".into(),
        max_units: 0,
        params: vec![Param::new(ParamType::String, "./tests/fixtures/token.wasm")],
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
    let plan_responses = simulator.run(&plan).unwrap();

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
        .read_only(
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
        .read_only(
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
        .read_only(
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
fn test_create_program() {
    build(SIMULATOR_GO_PATH, SIMULATOR_BUILD_PATH).expect("build simulator");
    let simulator = simulator::Client::new(SIMULATOR_BUILD_PATH);

    let owner_key = "owner";
    // create owner key in single step
    let resp = simulator.key_create(owner_key, Key::Ed25519).unwrap();
    assert_eq!(resp.error, None);

    let p_path = "./tests/fixtures/token.wasm";
    // create a new program on chain.
    let resp = simulator.program_create("owner", p_path.as_ref()).unwrap();
    assert_eq!(resp.error, None);
    assert_eq!(resp.result.id.is_some(), true);
}
