use std::env;

use wasmlanche_sdk::simulator::{
    id_from_step, Client, Endpoint, Key, Param, ParamType, Plan, PlanResponse, Step,
};

/// The following is an example of generating a test plan for a single NFT.
fn main() -> Result<(), Box<dyn std::error::Error>> {
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
    let client = Client::new("path to simulator".to_owned());

    let tx = client.run::<PlanResponse>(&plan)?;
    tx.iter().all(|resp| resp.error.is_none());

    Ok(())
}

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
