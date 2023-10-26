use serial_test::serial;
use std::env;
use wasmlanche_sdk::simulator::{
    self, id_from_step, Endpoint, Key, Param, ParamType, Plan, PlanResponse, Step,
};

pub fn initialize_plan(
    nft_name: String,
    nft_name_length: String,
    nft_symbol: String,
    nft_symbol_length: String,
    nft_uri: String,
    nft_uri_length: String,
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
#[serial]
#[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
fn test_single_nft_plan() {
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

    let plan = initialize_plan(
        nft_name,
        nft_name_length,
        nft_symbol,
        nft_symbol_length,
        nft_uri,
        nft_uri_length,
    );

    // run plan
    let plan_responses = simulator.run::<PlanResponse>(&plan).unwrap();

    assert!(
        plan_responses.iter().all(|resp| resp.error.is_none()),
        "error: {:?}",
        plan_responses
            .iter()
            .filter_map(|resp| resp.error.as_ref())
            .next()
    );
}

#[test]
#[serial]
#[ignore = "requires SIMULATOR_PATH and PROGRAM_PATH to be set"]
fn test_create_nft_program() {
    let s_path = env::var(simulator::PATH_KEY).expect("SIMULATOR_PATH not set");
    let simulator = simulator::Client::new(s_path);

    let alice_key = "alice_key";
    // create alice key in single step
    let resp = simulator
        .key_create::<PlanResponse>(alice_key, Key::Ed25519)
        .unwrap();
    assert_eq!(resp.error, None);

    let p_path = env::var("PROGRAM_PATH").expect("PROGRAM_PATH not set");
    // create a new program on chain.
    let resp = simulator
        .program_create::<PlanResponse>("owner", p_path.as_ref())
        .unwrap();
    assert_eq!(resp.error, None);
    assert!(resp.result.id.is_some());
}
