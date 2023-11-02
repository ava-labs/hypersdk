use std::env;
use wasmlanche_sdk::simulator::{
    id_from_step, Endpoint, Key, Operator, Param, ParamType, Plan, PlanResponse, Require,
    ResultAssertion, Step,
};

pub fn initialize_plan(
    nft_name: String,
    nft_name_length: String,
    nft_symbol: String,
    nft_symbol_length: String,
    nft_uri: String,
    nft_uri_length: String,
    nft_max_supply: String,
) -> Plan {
    let p_path = env::var("PROGRAM_PATH").expect("PROGRAM_PATH not set");
    let steps = vec![
        Step {
            endpoint: Endpoint::Execute,
            method: "program_create".to_string(),
            max_units: 0,
            params: vec![Param {
                param_type: ParamType::String,
                value: p_path,
            }],
            require: None,
        },
        Step {
            endpoint: Endpoint::Execute,
            method: "init".to_string(),
            max_units: 100000,
            params: vec![
                Param {
                    param_type: ParamType::Id,
                    value: id_from_step(0),
                },
                Param {
                    param_type: ParamType::String,
                    value: nft_name,
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_name_length,
                },
                Param {
                    param_type: ParamType::String,
                    value: nft_symbol,
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_symbol_length,
                },
                Param {
                    param_type: ParamType::String,
                    value: nft_uri,
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_uri_length,
                },
                Param {
                    param_type: ParamType::U64,
                    value: nft_max_supply,
                },
            ],
            require: None,
        },
        Step {
            endpoint: Endpoint::Key,
            method: "key_create".to_string(),
            max_units: 0,
            params: vec![Param {
                param_type: ParamType::Key(Key::Ed25519),
                value: "alice_key".to_string(),
            }],
            require: None,
        },
        Step {
            endpoint: Endpoint::Execute,
            method: "mint".to_string(),
            max_units: 100000,
            params: vec![
                Param {
                    param_type: ParamType::Id,
                    value: id_from_step(0),
                },
                Param {
                    param_type: ParamType::Key(Key::Ed25519),
                    value: "alice_key".to_string(),
                },
                Param {
                    param_type: ParamType::U64,
                    value: "1".to_string(),
                },
            ],
            require: None,
        },
        Step {
            endpoint: Endpoint::Execute,
            method: "burn".to_string(),
            max_units: 100000,
            params: vec![
                Param {
                    param_type: ParamType::Id,
                    value: id_from_step(0),
                },
                Param {
                    param_type: ParamType::Key(Key::Ed25519),
                    value: "alice_key".to_string(),
                },
                Param {
                    param_type: ParamType::U64,
                    value: "1".to_string(),
                },
            ],
            require: None,
        },
    ];

    Plan {
        caller_key: "alice_key".to_string(),
        steps,
    }
}

// export SIMULATOR_PATH=/path/to/simulator
// export PROGRAM_PATH=/path/to/program.wasm
#[test]
fn test_nft_plan() {
    use wasmlanche_sdk::simulator::{self, Key};
    let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
    let simulator = simulator::Client::new(s_path);

    let alice_key = "alice_key";
    // create owner key in single step
    let resp = simulator
        .key_create::<PlanResponse>(alice_key, Key::Ed25519)
        .unwrap();
    assert_eq!(resp.error, None);

    // create multiple step test plan
    let nft_name = "MyNFT".to_string();
    let binding = nft_name.len().to_string();
    let nft_name_length = binding.to_string();

    let nft_symbol = "MNFT".to_string();
    let binding = nft_symbol.len().to_string();
    let nft_symbol_length = binding.to_string();

    let nft_uri = "ipfs://my-nft.jpg".to_string();
    let binding = nft_uri.len().to_string();
    let nft_uri_length = binding.to_string();

    let nft_max_supply = "10".to_string();

    let plan = initialize_plan(
        nft_name,
        nft_name_length,
        nft_symbol,
        nft_symbol_length,
        nft_uri,
        nft_uri_length,
        nft_max_supply,
    );

    // run plan
    let plan_responses = simulator.run::<PlanResponse>(&plan).unwrap();

    // collect actual id of program from step 0
    let mut program_id = String::new();
    if let Some(step_0) = plan_responses.first() {
        program_id = step_0.result.id.clone().unwrap_or_default();
    }

    assert!(
        plan_responses.iter().all(|resp| resp.error.is_none()),
        "error: {:?}",
        plan_responses
            .iter()
            .filter_map(|resp| resp.error.as_ref())
            .next()
    );

    // Check Alice balance is 0 as expected after minting 1 and burning 1.
    let resp = simulator
        .read_only::<PlanResponse>(
            "alice_key",
            "balance",
            vec![
                Param::new(ParamType::Id, program_id.as_ref()),
                Param::new(ParamType::Key(Key::Ed25519), "alice_key"),
            ],
            Some(Require {
                result: ResultAssertion {
                    operator: Operator::NumericEq,
                    value: "0".into(),
                },
            }),
        )
        .expect("failed to get alice balance");

    assert_eq!(resp.error, None);
}
